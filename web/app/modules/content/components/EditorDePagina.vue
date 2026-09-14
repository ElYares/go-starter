<script setup lang="ts">
// El editor de una pagina (CU-004). Sin nada de Nuxt, como ListaDeSettings:
// recibe las operaciones como funciones y no sabe de donde salen, asi se prueba
// cada camino —guardar, el 400, el 409, publicar, revertir— con promesas a mano.
//
// Tres reglas que no se ven en el marcado:
//
// - **Lo escrito no se pierde en un error.** Un 400 marca campos y un 409 avisa;
//   ninguno de los dos toca el borrador
// - **Un 409 no guarda encima.** Ofrece descartar y recargar, y dice cuando se
//   guardo la otra version. Sin esto, la ultima en guardar borra el trabajo de
//   la otra en silencio
// - **No se publica con cambios sin guardar.** Publicar apunta a una version
//   guardada; con cambios pendientes, el boton publicaria algo distinto de lo
//   que se ve en pantalla
import { computed, onMounted, ref, watch } from 'vue'
import {
  BaseBadge,
  BaseButton,
  BaseDialog,
  BaseEmptyState,
  BaseField,
  BaseInput,
  BaseSelect,
  BaseTextarea,
  BaseToast,
} from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { esquemas as esquemasDelCatalogo, type Esquema } from '~/shared/blocks/catalogo'
import { bloques as registro } from '~/shared/blocks/registry'
import RenderDeBloques from '~/shared/blocks/RenderDeBloques.vue'
import {
  borradorDe,
  cuerpoDeGuardado,
  erroresPorCampo,
  estadoDePublicacion,
  hayCambios,
  mover,
  nuevoBloque,
  type Bloque,
  type Borrador,
} from '../editor'
import BloqueEnEdicion from './BloqueEnEdicion.vue'
import HistorialDeVersiones from './HistorialDeVersiones.vue'

type Pagina = Schemas['Pagina']

const props = withDefaults(
  defineProps<{
    cargar: () => Promise<Pagina>
    guardar: (version: number, cuerpo: Schemas['PaginaModificacion']) => Promise<Pagina>
    publicar: (versionId: string) => Promise<Pagina>
    borrar: () => Promise<void>
    cargarVersiones: () => Promise<Schemas['VersionesPage']>
    puedeEscribir: boolean
    puedePublicar: boolean
    esquemas?: Record<string, Esquema>
  }>(),
  { esquemas: () => esquemasDelCatalogo },
)

const emit = defineEmits<{
  borrada: []
  /** Si hay cambios sin guardar, para que la vista avise antes de salir. */
  cambios: [pendientes: boolean]
}>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const fallo = ref<ApiError | null>(null)
const pagina = ref<Pagina | null>(null)
const borrador = ref<Borrador | null>(null)

const errores = ref<Record<string, string>>({})
const aviso = ref<{ titulo: string; detalle?: string; traceId?: string } | null>(null)
const conflicto = ref<{ guardadaEl?: string } | null>(null)

const guardando = ref(false)
const publicando = ref<string | null>(null)
const borrando = ref(false)
const confirmarBorrado = ref(false)
const recargaDelHistorial = ref(0)
const toast = ref({ abierto: false, titulo: '' })

const tipoAAgregar = ref<string>()

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    const leida = await props.cargar()
    pagina.value = leida
    borrador.value = borradorDe(leida)
    errores.value = {}
    aviso.value = null
    conflicto.value = null
    estado.value = 'listo'
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    fallo.value = causa
    estado.value = 'error'
  }
}

onMounted(traer)

const pendientes = computed(() => (pagina.value && borrador.value ? hayCambios(pagina.value, borrador.value) : false))
watch(pendientes, (v) => emit('cambios', v))

const publicacion = computed(() => (pagina.value ? estadoDePublicacion(pagina.value) : null))

const opcionesDeTipo = computed(() =>
  Object.keys(props.esquemas)
    .sort()
    .map((tipo) => ({ value: tipo, label: registro[tipo]?.label ?? tipo })),
)

const hayDesconocidos = computed(() => borrador.value?.blocks.some((b) => !props.esquemas[b.type]) ?? false)
const cuantosErrores = computed(() => Object.keys(errores.value).length)

function avisar(causa: unknown, que: string) {
  if (!(causa instanceof ApiError)) throw causa
  if (causa.status === 403) {
    aviso.value = { titulo: `No tienes permiso para ${que}`, traceId: causa.traceId }
  } else if (causa.unavailable) {
    aviso.value = {
      titulo: 'El servidor no responde',
      detalle: 'Tus cambios siguen aqui. Vuelve a intentarlo en un momento.',
      traceId: causa.traceId,
    }
  } else {
    aviso.value = { titulo: `No se pudo ${que}`, detalle: causa.message, traceId: causa.traceId }
  }
}

function avisarExito(titulo: string) {
  toast.value = { abierto: true, titulo }
}

async function onGuardar() {
  if (!pagina.value || !borrador.value) return
  guardando.value = true
  errores.value = {}
  aviso.value = null
  conflicto.value = null
  try {
    const guardada = await props.guardar(pagina.value.version, cuerpoDeGuardado(borrador.value))
    pagina.value = guardada
    borrador.value = borradorDe(guardada)
    recargaDelHistorial.value++
    avisarExito(`Guardada la version ${guardada.draftNumber}`)
  } catch (causa) {
    if (causa instanceof ApiError && causa.status === 400) {
      errores.value = erroresPorCampo(causa)
      const n = cuantosErrores.value
      aviso.value = {
        titulo: n === 0 ? 'La pagina tiene datos que no se aceptan' : `Hay ${n} ${n === 1 ? 'campo' : 'campos'} por corregir`,
        detalle: 'No se guardo nada. Lo que escribiste sigue aqui.',
        traceId: causa.traceId,
      }
    } else if (causa instanceof ApiError && causa.status === 409) {
      // Se lee la version actual solo para decir cuando se guardo; NO se
      // aplica sobre lo que la persona tiene en pantalla.
      conflicto.value = {}
      props
        .cargar()
        .then((actual) => {
          if (conflicto.value) conflicto.value = { guardadaEl: actual.updatedAt }
        })
        .catch(() => {})
    } else {
      avisar(causa, 'guardar la pagina')
    }
  } finally {
    guardando.value = false
  }
}

async function onPublicar(versionId: string, numero: number) {
  if (!pagina.value) return
  publicando.value = versionId
  aviso.value = null
  try {
    const publicada = await props.publicar(versionId)
    // El borrador no cambia al publicar: solo el puntero. Se toma lo que dice
    // el servidor de la publicacion y se deja lo que hay en pantalla.
    pagina.value = { ...pagina.value, ...pick(publicada) }
    recargaDelHistorial.value++
    avisarExito(`Publicada la version ${numero}`)
  } catch (causa) {
    avisar(causa, 'publicar')
  } finally {
    publicando.value = null
  }
}

function pick(p: Pagina) {
  return {
    publishedVersionId: p.publishedVersionId,
    publishedNumber: p.publishedNumber,
    updatedAt: p.updatedAt,
    updatedBy: p.updatedBy,
  }
}

async function onBorrar() {
  borrando.value = true
  try {
    await props.borrar()
    confirmarBorrado.value = false
    emit('borrada')
  } catch (causa) {
    confirmarBorrado.value = false
    avisar(causa, 'borrar la pagina')
  } finally {
    borrando.value = false
  }
}

function agregarBloque() {
  const tipo = tipoAAgregar.value
  const esquema = tipo ? props.esquemas[tipo] : undefined
  if (!borrador.value || !tipo || !esquema) return
  borrador.value.blocks = [...borrador.value.blocks, nuevoBloque(tipo, esquema, borrador.value.blocks)]
}

function actualizarBloque(i: number, bloque: Bloque) {
  if (!borrador.value) return
  const copia = [...borrador.value.blocks]
  copia[i] = bloque
  borrador.value.blocks = copia
}

function moverBloque(i: number, delta: -1 | 1) {
  if (borrador.value) borrador.value.blocks = mover(borrador.value.blocks, i, delta)
}

function quitarBloque(i: number) {
  if (borrador.value) borrador.value.blocks = borrador.value.blocks.filter((_, j) => j !== i)
}

const fecha = (iso?: string) => (iso ? iso.slice(0, 16).replace('T', ' ') : '')
</script>

<template>
  <section class="editor" :aria-busy="estado === 'cargando' || undefined">
    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando la pagina">
      <div class="fila-fantasma alta" />
      <div v-for="n in 3" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="
          fallo?.status === 403
            ? 'No tienes permiso para ver esta pagina'
            : fallo?.status === 404
              ? 'Esta pagina no existe'
              : fallo?.unavailable
                ? 'El servidor no responde'
                : 'No se pudo cargar la pagina'
        "
        :description="
          fallo?.status === 403
            ? 'Pidele a quien administra el sitio el permiso content.page.read.'
            : fallo?.status === 404
              ? 'Puede que alguien la haya borrado.'
              : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.'
        "
      >
        <template #accion>
          <BaseButton v-if="fallo?.status !== 403 && fallo?.status !== 404" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="pista centrada">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>

    <div v-else-if="pagina && borrador" class="listo" data-estado="listo">
      <header class="cabecera">
        <div>
          <h1>{{ pagina.title }}</h1>
          <p class="pista">
            <code>/{{ pagina.slug === 'inicio' ? '' : pagina.slug }}</code> · borrador version {{ pagina.draftNumber }}
          </p>
        </div>
        <div class="estado-publicacion">
          <BaseBadge v-if="publicacion === 'sin-publicar'" variant="neutral" sr-label="Publicacion">Sin publicar</BaseBadge>
          <BaseBadge v-else-if="publicacion === 'al-dia'" variant="ok" sr-label="Publicacion">
            Publicada · version {{ pagina.publishedNumber }}
          </BaseBadge>
          <BaseBadge v-else variant="accent" sr-label="Publicacion">
            Publicada la version {{ pagina.publishedNumber }} · hay cambios sin publicar
          </BaseBadge>
          <BaseBadge v-if="pendientes" variant="danger" sr-label="Cambios">Sin guardar</BaseBadge>
        </div>
      </header>

      <div class="barra" role="toolbar" aria-label="Acciones de la pagina">
        <BaseButton v-if="puedeEscribir" :loading="guardando" :disabled="!pendientes" @click="onGuardar">
          Guardar
        </BaseButton>
        <BaseButton
          v-if="puedePublicar"
          variant="secondary"
          :loading="publicando === pagina.draftVersionId"
          :disabled="pendientes || publicacion === 'al-dia' || publicando !== null"
          :title="pendientes ? 'Guarda antes de publicar' : undefined"
          @click="onPublicar(pagina.draftVersionId, pagina.draftNumber)"
        >
          Publicar version {{ pagina.draftNumber }}
        </BaseButton>
        <span v-if="puedePublicar && pendientes" class="pista">Guarda antes de publicar.</span>
        <span class="espacio" />
        <BaseButton v-if="puedeEscribir" variant="danger" @click="confirmarBorrado = true">Borrar pagina</BaseButton>
      </div>

      <p v-if="!puedeEscribir" class="pista" data-aviso="solo-lectura">
        Solo lectura: para editar hace falta el permiso content.page.write.
      </p>

      <div v-if="conflicto" class="aviso" role="alert" data-aviso="conflicto">
        <strong>Alguien guardo esta pagina mientras la editabas.</strong>
        <p>
          Se guardo otra version{{ conflicto.guardadaEl ? ` el ${fecha(conflicto.guardadaEl)}` : '' }}. No se guardo nada
          encima: tus cambios siguen en pantalla. Para seguir, descartalos y recarga la version actual.
        </p>
        <BaseButton variant="secondary" size="sm" @click="traer">Descartar mis cambios y recargar</BaseButton>
      </div>

      <div v-if="aviso" class="aviso" role="alert" data-aviso="error">
        <strong>{{ aviso.titulo }}</strong>
        <p v-if="aviso.detalle">{{ aviso.detalle }}</p>
        <p v-if="aviso.traceId" class="pista">Referencia: <code>{{ aviso.traceId }}</code></p>
      </div>

      <div class="columnas">
        <div class="columna">
          <section class="datos">
            <h2>Datos de la pagina</h2>
            <BaseField label="Titulo" :error="errores.title" required>
              <template #default="{ id, describedBy, invalid }">
                <BaseInput :id="id" v-model="borrador.title" :described-by="describedBy" :invalid="invalid" :disabled="!puedeEscribir" data-campo="title" />
              </template>
            </BaseField>
            <BaseField label="Slug" :error="errores.slug" hint="La direccion de la pagina. Cambiarlo cambia su URL publica al guardar." required>
              <template #default="{ id, describedBy, invalid }">
                <BaseInput :id="id" v-model="borrador.slug" :described-by="describedBy" :invalid="invalid" :disabled="!puedeEscribir" data-campo="slug" />
              </template>
            </BaseField>
            <BaseField label="Titulo para buscadores" :error="errores.seoTitle" hint="Si lo dejas vacio se usa el titulo.">
              <template #default="{ id, describedBy, invalid }">
                <BaseInput :id="id" v-model="borrador.seoTitle" :described-by="describedBy" :invalid="invalid" :disabled="!puedeEscribir" data-campo="seoTitle" />
              </template>
            </BaseField>
            <BaseField label="Descripcion para buscadores" :error="errores.seoDescription">
              <template #default="{ id, describedBy, invalid }">
                <BaseTextarea :id="id" v-model="borrador.seoDescription" :rows="2" :described-by="describedBy" :invalid="invalid" :disabled="!puedeEscribir" data-campo="seoDescription" />
              </template>
            </BaseField>
          </section>

          <section class="bloques">
            <h2>Bloques</h2>
            <p v-if="hayDesconocidos" class="aviso-suave" data-aviso="desconocidos">
              Esta pagina tiene bloques de un tipo que ya no existe. Quitalos para poder guardar.
            </p>
            <p v-if="borrador.blocks.length === 0" class="pista">
              La pagina no tiene bloques. Agrega el primero para que tenga contenido.
            </p>
            <BloqueEnEdicion
              v-for="(bloque, i) in borrador.blocks"
              :key="bloque.id"
              :bloque="bloque"
              :indice="i"
              :total="borrador.blocks.length"
              :esquema="esquemas[bloque.type]"
              :etiqueta="registro[bloque.type]?.label"
              :errores="errores"
              :deshabilitado="!puedeEscribir"
              @update:bloque="actualizarBloque(i, $event)"
              @subir="moverBloque(i, -1)"
              @bajar="moverBloque(i, 1)"
              @quitar="quitarBloque(i)"
            />

            <div v-if="puedeEscribir" class="agregar">
              <BaseField label="Tipo de bloque">
                <template #default="{ id, describedBy }">
                  <BaseSelect :id="id" v-model="tipoAAgregar" :options="opcionesDeTipo" :described-by="describedBy" placeholder="Elegir tipo…" />
                </template>
              </BaseField>
              <BaseButton variant="secondary" :disabled="!tipoAAgregar" @click="agregarBloque">Agregar bloque</BaseButton>
            </div>

            <BaseField v-if="puedeEscribir" label="Nota de este guardado" :error="errores.note" hint="Opcional. Aparece en el historial.">
              <template #default="{ id, describedBy, invalid }">
                <BaseInput :id="id" v-model="borrador.note" :described-by="describedBy" :invalid="invalid" data-campo="note" />
              </template>
            </BaseField>
          </section>
        </div>

        <div class="columna">
          <section class="vista-previa" aria-label="Vista previa">
            <h2>Vista previa</h2>
            <p class="pista">Asi se vera al publicar lo que tienes en pantalla.</p>
            <div class="lienzo">
              <RenderDeBloques :bloques="borrador.blocks" />
            </div>
          </section>

          <HistorialDeVersiones
            :cargar="cargarVersiones"
            :puede-publicar="puedePublicar"
            :recarga="recargaDelHistorial"
            :publicando="publicando"
            @publicar="onPublicar"
          />
        </div>
      </div>

      <BaseDialog v-model:open="confirmarBorrado" title="Borrar la pagina" :description="`Se borran la pagina /${pagina.slug} y todas sus versiones. Si estaba publicada, su URL deja de funcionar. No se puede deshacer.`">
        <template #acciones>
          <BaseButton variant="secondary" @click="confirmarBorrado = false">Cancelar</BaseButton>
          <BaseButton variant="danger" :loading="borrando" @click="onBorrar">Borrar para siempre</BaseButton>
        </template>
      </BaseDialog>

      <BaseToast v-model:open="toast.abierto" :title="toast.titulo" variant="ok" />
    </div>
  </section>
</template>

<style scoped>
h1 {
  margin: 0;
  font-size: var(--text-lg);
}
h2 {
  margin: 0 0 var(--space-3);
  font-size: var(--text-base);
}
.esqueleto {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.fila-fantasma {
  height: var(--space-8);
  border-radius: var(--radius-sm);
  background: var(--color-surface-hover);
}
.fila-fantasma.alta {
  height: calc(var(--space-8) * 2);
}
.listo {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.cabecera {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  align-items: flex-start;
  justify-content: space-between;
}
.estado-publicacion,
.barra {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}
.espacio {
  flex: 1;
}
.aviso {
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.aviso p {
  margin: var(--space-1) 0 var(--space-2);
}
.aviso-suave {
  margin: 0 0 var(--space-3);
  color: var(--color-danger);
  font-size: var(--text-sm);
}
.columnas {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(22rem, 1fr));
  gap: var(--space-6);
  align-items: start;
}
.columna,
.datos,
.bloques {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.agregar {
  display: flex;
  gap: var(--space-2);
  align-items: flex-end;
}
.lienzo {
  padding: var(--space-4);
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-bg);
}
.pista {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.centrada {
  text-align: center;
}
code {
  font-family: var(--font-mono);
}
</style>
