# 04 · Reglas de CRUD — el molde

Todo recurso del proyecto se comporta igual. No porque sea elegante, sino
porque un starter que se forkea diez veces no puede tener diez formas de
paginar. **Este documento es normativo:** un endpoint que no lo cumple no está
hecho, aunque funcione.

## 1. Rutas

```
GET    /api/v1/pages            listar
POST   /api/v1/pages            crear
GET    /api/v1/pages/{id}       leer
PUT    /api/v1/pages/{id}       reemplazar
PATCH  /api/v1/pages/{id}       modificar parcialmente (solo si hace falta)
DELETE /api/v1/pages/{id}       borrar
POST   /api/v1/pages/{id}/publish   acción que no es CRUD
```

- Plural, `kebab-case`, siempre bajo `/api/v1`
- **Un solo nivel de anidamiento**: `/pages/{id}/versions` sí;
  `/sites/{s}/pages/{p}/versions/{v}` no. A partir del segundo nivel, el hijo
  merece ruta propia
- Lo que **no** es CRUD va como verbo en un subrecurso (`/publish`, `/enable`),
  nunca como un `PATCH` con un campo mágico que dispara efectos
- Lo público y sin sesión vive bajo `/api/v1/public/…`, separado a propósito:
  así una ruta pública nunca aparece por descuido

## 2. Códigos de respuesta

| Situación | Código | Cuerpo |
|---|---|---|
| Lectura correcta | `200` | el recurso o la colección |
| Creación | `201` + `Location` | el recurso creado |
| Creación que ya existía (dedup) | `200` | el recurso existente |
| Actualización | `200` | el recurso actualizado |
| Borrado | `204` | vacío |
| Cuerpo o parámetros inválidos | `400` `VALIDATION_FAILED` | problem+json con `errors[]` |
| Sin sesión | `401` `UNAUTHENTICATED` | problem+json |
| Con sesión, sin permiso | `403` `FORBIDDEN` | problem+json |
| No existe **o no es tuyo** | `404` `NOT_FOUND` | problem+json |
| Conflicto de estado o de versión | `409` `CONFLICT` | problem+json |
| Demasiadas peticiones | `429` + `Retry-After` | problem+json |

**`404` y no `403` cuando el recurso es de otro.** Un `403` confirma que el
recurso existe, y con eso se enumeran ids ajenos.

## 3. Listar

Respuesta, siempre esta forma:

```json
{
  "content": [ … ],
  "page": { "number": 0, "size": 20, "totalElements": 137, "totalPages": 7 }
}
```

- Nunca un array pelado en la raíz: agregar metadatos después rompe a todos los
  clientes a la vez
- `size` por omisión **20**, tope duro **100**. Sin el tope, `?size=1000000` es
  una negación de servicio de una línea, y `page.size` debe reportar el tamaño
  **efectivo**, no el pedido
- `?sort=campo,dir` con **lista blanca por recurso**. Un campo fuera de la lista
  es `400`, no un error de SQL filtrado como `500`
- El orden se aplica **en la consulta**, jamás sobre la página ya traída
- Todo orden termina con un desempate estable (`, id asc`). Sin él, dos filas
  con el mismo `created_at` se repiten o se pierden entre páginas
- Filtros: **lista blanca declarada por recurso**, mapeada a columnas en
  servidor. Ningún fragmento de SQL viaja desde el cliente, nunca
- Paginación por `offset`. Cuando un recurso pase el millón de filas, ese
  recurso —y solo ese— migra a cursor

## 4. Crear y actualizar

- **Se valida en el borde, se decide en el service, se escribe en el repo.** El
  handler no tiene `if` de negocio; el service no arma SQL
- El cuerpo se decodifica a un DTO de entrada explícito. Nunca directo a la
  entidad: eso es *mass assignment* y regala `id`, `createdBy` y `roles`
- Campos desconocidos en el JSON → `400`. Aceptar un `titel` en silencio
  significa que el usuario cree que guardó algo que no guardó
- `PUT` reemplaza el recurso completo; `PATCH` solo existe si hay un caso real
  de edición parcial, y entonces distingue **ausente** de **null**
- Toda escritura corre en una transacción, incluida la auditoría

### Concurrencia

Todo recurso editable desde el dashboard lleva `version`:

```
GET  /pages/{id}   →  200, ETag: "7"
PUT  /pages/{id}   con  If-Match: "7"
       version en base == 7  →  200, ETag: "8"
       version en base == 9  →  409 CONFLICT
```

Sin esto, dos personas editando la misma página producen que la última en
guardar borre el trabajo de la otra, en silencio y sin rastro. Es el bug que
nadie reporta porque nadie sabe que pasó.

### Idempotencia

Un `POST` que crea algo con efecto externo (un pago, un correo, una orden)
acepta `Idempotency-Key`. La clave y la respuesta se guardan; repetir la misma
clave devuelve la respuesta original sin volver a ejecutar. Un reintento de red
no debe cobrar dos veces.

## 5. Autorización

Dos preguntas distintas, y hay que contestar las dos:

1. **¿Puedes hacer esta operación?** → permiso (`content.page.publish`),
   declarado por el módulo y verificado por el guard de la ruta
2. **¿Es tuyo este recurso?** → propiedad, filtrada **en la consulta**:

```sql
select … from pages where id = $1 and (created_by = $2 or $3 /* es admin */)
```

**Nunca un `if` después de cargar la fila.** Un `if` se olvida en el endpoint
número quince; un método de repositorio que no compila sin el actor, no. El
guard cierra por omisión: una ruta sin política declarada se rechaza, no se
permite.

## 6. Errores

`Content-Type: application/problem+json` (RFC 7807), con dos campos añadidos:

```json
{
  "type": "about:blank",
  "title": "Validación fallida",
  "status": 400,
  "detail": "El cuerpo tiene 2 campos inválidos",
  "instance": "/api/v1/pages",
  "code": "VALIDATION_FAILED",
  "traceId": "01J8…",
  "errors": [
    { "field": "slug",  "code": "required", "message": "El slug es obligatorio" },
    { "field": "title", "code": "maxLength", "message": "Máximo 120 caracteres" }
  ]
}
```

- `code` es **estable y en mayúsculas**: es lo que el frontend interpreta. El
  `message` es para humanos y puede cambiar sin romper nada
- `traceId` viaja en la respuesta **y** en el log. Es lo que convierte "no me
  deja guardar" en un log encontrable
- Un `500` nunca expone el error interno. Se registra completo y se responde
  genérico con su `traceId`

## 7. Auditoría

`created_at`/`created_by`/`updated_at`/`updated_by` los llena `platform/audit`
leyendo el actor del contexto de la petición. El código de negocio no los
asigna. Operaciones sin actor (seed, tarea de fondo) usan un actor de sistema
explícito, no `null` por descuido.

## 8. Checklist: un CRUD está hecho cuando

- [ ] Las seis rutas existen o está escrito por qué alguna no
- [ ] Está en `openapi.yaml` y los tipos del frontend están regenerados
- [ ] Listar respeta el envoltorio, el tope de `size` y la lista blanca de orden
- [ ] Hay una prueba de que `?size=1000000` devuelve 100
- [ ] Hay una prueba de que un `sort` inválido da `400` y no `500`
- [ ] Hay una prueba de que el usuario A recibe `404` sobre un recurso de B
- [ ] Hay una prueba de que sin permiso da `403` y sin sesión da `401`
- [ ] Hay una prueba de que un `If-Match` viejo da `409`
- [ ] Los errores salen como problem+json con `code` y `traceId`
- [ ] La auditoría se llena sola, verificado en una prueba de integración
- [ ] El frontend cubre los cuatro estados de la vista (`07-frontend.md`)

Si el recurso es de un fork, este checklist se copia tal cual. Ese es el punto.
