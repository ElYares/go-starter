# go-starter

Starter propio de Go + Nuxt: una landing publica editable desde un dashboard
administrativo, construido para forkearse hacia distintas reglas de negocio
(tienda, landing, panel administrativo) sin reescribir la base.

## Documentacion tecnica

`docs/INDEX.md` es el indice: leerlo antes de abrir cualquier otro doc.

## Contexto del proyecto

Estado, decisiones e historia en el vault:
`~/develop/docs/obsidean-vault-personal/10 Projects/go-starter/`
Cargar con `/contexto`.

## Reglas que no se negocian

- La plataforma (`internal/platform/`) no importa modulos. Nunca. Ver
  `docs/01-arquitectura.md`
- Un CRUD nuevo sigue el molde de `docs/04-reglas-de-crud.md` completo, o no
  esta hecho
- El contrato manda: se edita `api/openapi.yaml` y se regeneran tipos, no al
  reves. Ver `docs/05-contratos-api.md`
