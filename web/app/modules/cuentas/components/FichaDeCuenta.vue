<script setup lang="ts">
// La ficha de una cuenta (HU-018): sus datos, si puede entrar y sus roles. Sin
// Nuxt, como el editor de paginas: recibe las operaciones como funciones, asi
// cada camino —el 400, los dos 409, el ultimo superadmin— se prueba a mano.
//
// Reglas que no se ven en el marcado:
//
// - **Lo escrito no se pierde en un error**, como en el editor de paginas
// - **Un 409 al guardar puede ser dos cosas**: otra version o un correo que ya
//   usa otra cuenta. El codigo es el mismo (`CONFLICT`), asi que se relee: si la
//   version no cambio, nadie guardo en medio y el conflicto es el correo. No se
//   decide leyendo el `detail`, que es texto para humanos y puede cambiar
// - **El ultimo superadmin** llega como 409 al quitarle el rol o deshabilitarlo,
//   y se muestra con el mensaje del servidor, que dice que hacer
// - **Deshabilitar pide confirmacion**: cierra todas las sesiones de la cuenta
import { computed, onMounted, ref, watch } from 'vue'
import { BaseBadge, BaseButton, BaseDialog, BaseEmptyState, BaseField, BaseInput, BaseToast } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { erroresPorCampo } from '~/shared/formularios/esquema'

type Cuenta = Schemas['Cuenta']

const props = defineProps<{
  cargar: () => Promise<Cuenta>
  guardar: (version: number, cuerpo: Schemas['CuentaModificacion']) => Promise<Cuenta>
  deshabilitar: () => Promise<Cuenta>
  habilitar: () => Promise<Cuenta>
  darRol: (rol: string) => Promise<void>
  quitarRol: (rol: string) => Promise<void>
  /** El catalogo de roles. Sin `identity.role.read` no se pasa y se ven solo los que tiene. */
  cargarRoles?: () => Promise<Schemas['RolesPage']>
  puedeEscribir: boolean
  puedeAsignar: boolean
  /** Si es la cuenta de quien mira: deshabilitarla lo saca a el. */
  esPropia: boolean
}>()

const emit = defineEmits<{ cambios: [pendientes: boolean] }>()

const estado = ref<'cargando' | 'error' | 'listo'>('cargando')
const fallo = ref<ApiError | null>(null)
const cuenta = ref<Cuenta | null>(null)
const datos = ref({ email: '', displayName: '' })
const catalogo = ref<Schemas['Rol'][]>([])

const errores = ref<Record<string, string>>({})
const aviso = ref<{ titulo: string; detalle?: string; traceId?: string } | null>(null)
const conflicto = ref<{ guardadaEl?: string } | null>(null)
const guardando = ref(false)
const cambiandoAcceso = ref(false)
const cambiandoRol = ref<string | null>(null)
const confirmarBaja = ref(false)
const toast = ref({ abierto: false, titulo: '' })

function tomar(c: Cuenta) {
  cuenta.value = c
  datos.value = { email: c.email, displayName: c.displayName }
}

async function traer() {
  estado.value = 'cargando'
  fallo.value = null
  try {
    // El catalogo es conveniencia: si falla, la ficha sigue mostrando los
    // roles que la cuenta tiene, sin casillas para los demas.
    const [leida, roles] = await Promise.all([
      props.cargar(),
      props.cargarRoles ? props.cargarRoles().catch(() => null) : Promise.resolve(null),
    ])
    tomar(leida)
    catalogo.value = roles?.content ?? []
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

const pendientes = computed(() =>
  cuenta.value ? datos.value.email !== cuenta.value.email || datos.value.displayName !== cuenta.value.displayName : false,
)
watch(pendientes, (v) => emit('cambios', v))

/** Las claves a mostrar: todo el catalogo si se tiene, si no las que ya tiene. */
const rolesVisibles = computed(() => {
  const asignados = cuenta.value?.roles ?? []
  if (catalogo.value.length === 0) return asignados.map((key) => ({ key, name: key }))
  return catalogo.value.map((r) => ({ key: r.key, name: r.name }))
})

function avisar(causa: ApiError, que: string) {
  if (causa.status === 403) {
    aviso.value = { titulo: `No tienes permiso para ${que}`, traceId: causa.traceId }
  } else if (causa.status === 409) {
    // El unico 409 fuera del guardado es el ultimo superadmin; su mensaje dice
    // que hacer, asi que se muestra tal cual.
    aviso.value = { titulo: `No se pudo ${que}`, detalle: causa.message, traceId: causa.traceId }
  } else if (causa.unavailable) {
    aviso.value = { titulo: 'El servidor no responde', detalle: 'Vuelve a intentarlo en un momento.', traceId: causa.traceId }
  } else {
    aviso.value = { titulo: `No se pudo ${que}`, detalle: causa.message, traceId: causa.traceId }
  }
}

async function onGuardar() {
  if (!cuenta.value || !pendientes.value || guardando.value) return
  const version = cuenta.value.version
  guardando.value = true
  errores.value = {}
  aviso.value = null
  conflicto.value = null
  try {
    tomar(await props.guardar(version, { ...datos.value }))
    toast.value = { abierto: true, titulo: 'Cuenta guardada' }
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    if (causa.status === 400) {
      errores.value = erroresPorCampo(causa)
      aviso.value = { titulo: 'Hay datos que no se aceptan', detalle: 'No se guardo nada. Lo que escribiste sigue aqui.' }
    } else if (causa.status === 409) {
      await explicarConflicto(version)
    } else {
      avisar(causa, 'guardar la cuenta')
    }
  } finally {
    guardando.value = false
  }
}

// Ver el comentario de arriba: relee para saber de que 409 se trata. Lo leido
// NO se aplica sobre lo que hay en pantalla.
async function explicarConflicto(version: number) {
  try {
    const actual = await props.cargar()
    if (actual.version === version) {
      errores.value = { email: 'Ya hay otra cuenta con este correo.' }
    } else {
      conflicto.value = { guardadaEl: actual.updatedAt }
    }
  } catch {
    // Sin poder releer no se sabe cual fue; se dice lo mas prudente.
    conflicto.value = {}
  }
}

function descartar() {
  if (!cuenta.value) return
  tomar(cuenta.value)
  errores.value = {}
  aviso.value = null
}

async function cambiarAcceso(habilitar: boolean) {
  confirmarBaja.value = false
  cambiandoAcceso.value = true
  aviso.value = null
  try {
    const nueva = await (habilitar ? props.habilitar() : props.deshabilitar())
    // Solo el estado y la version: lo que la persona este escribiendo en el
    // formulario no se toca.
    if (cuenta.value) cuenta.value = { ...cuenta.value, enabled: nueva.enabled, version: nueva.version, updatedAt: nueva.updatedAt }
    toast.value = { abierto: true, titulo: habilitar ? 'Cuenta habilitada' : 'Cuenta deshabilitada' }
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    avisar(causa, habilitar ? 'habilitar la cuenta' : 'deshabilitar la cuenta')
  } finally {
    cambiandoAcceso.value = false
  }
}

async function alternarRol(rol: string, casilla: HTMLInputElement) {
  if (!cuenta.value) return
  const tener = casilla.checked
  cambiandoRol.value = rol
  aviso.value = null
  try {
    await (tener ? props.darRol(rol) : props.quitarRol(rol))
    const actuales = cuenta.value.roles.filter((r) => r !== rol)
    cuenta.value = { ...cuenta.value, roles: tener ? [...actuales, rol].sort() : actuales }
  } catch (causa) {
    // La casilla se devuelve a mano: `:checked` no cambio —el rol sigue como
    // estaba—, asi que Vue no tiene nada que repintar y el clic de la persona
    // quedaria en pantalla como si se hubiera aplicado.
    casilla.checked = !tener
    if (!(causa instanceof ApiError)) throw causa
    avisar(causa, tener ? `darle el rol ${rol}` : `quitarle el rol ${rol}`)
  } finally {
    cambiandoRol.value = null
  }
}

const fecha = (iso?: string) => (iso ? iso.slice(0, 16).replace('T', ' ') : '')
</script>

<template>
  <section class="ficha" :aria-busy="estado === 'cargando' || undefined">
    <div v-if="estado === 'cargando'" class="esqueleto" data-estado="cargando" aria-label="Cargando la cuenta">
      <div class="fila-fantasma alta" />
      <div v-for="n in 3" :key="n" class="fila-fantasma" />
    </div>

    <div v-else-if="estado === 'error'" data-estado="error">
      <BaseEmptyState
        :title="
          fallo?.status === 403
            ? 'No tienes permiso para ver esta cuenta'
            : fallo?.status === 404
              ? 'Esta cuenta no existe'
              : fallo?.unavailable
                ? 'El servidor no responde'
                : 'No se pudo cargar la cuenta'
        "
        :description="
          fallo?.status === 403
            ? 'Pidele a quien administra el sitio el permiso identity.user.read.'
            : fallo?.status === 404
              ? 'Puede que el enlace este mal copiado.'
              : 'Vuelve a intentarlo. Si se repite, comparte la referencia de abajo.'
        "
      >
        <template #accion>
          <BaseButton v-if="fallo?.status !== 403 && fallo?.status !== 404" @click="traer">Reintentar</BaseButton>
        </template>
      </BaseEmptyState>
      <p v-if="fallo?.traceId" class="pista centrada">Referencia: <code>{{ fallo.traceId }}</code></p>
    </div>

    <div v-else-if="cuenta" class="listo" data-estado="listo">
      <header class="cabecera">
        <div>
          <h1>{{ cuenta.displayName }}</h1>
          <p class="pista">{{ cuenta.email }}</p>
        </div>
        <div class="insignias">
          <BaseBadge :variant="cuenta.enabled ? 'ok' : 'danger'" sr-label="Acceso" data-insignia="acceso">
            {{ cuenta.enabled ? 'Habilitada' : 'Deshabilitada' }}
          </BaseBadge>
          <BaseBadge v-if="esPropia" variant="accent">Tu cuenta</BaseBadge>
          <BaseBadge v-if="pendientes" variant="danger" sr-label="Cambios">Sin guardar</BaseBadge>
        </div>
      </header>

      <div v-if="conflicto" class="aviso" role="alert" data-aviso="conflicto">
        <strong>Alguien guardo esta cuenta mientras la editabas.</strong>
        <p>
          Se guardo otra version{{ conflicto.guardadaEl ? ` el ${fecha(conflicto.guardadaEl)}` : '' }}. No se guardo
          nada encima: tus cambios siguen en pantalla. Para seguir, descartalos y recarga la version actual.
        </p>
        <BaseButton variant="secondary" size="sm" @click="traer">Descartar mis cambios y recargar</BaseButton>
      </div>

      <div v-if="aviso" class="aviso" role="alert" data-aviso="error">
        <strong>{{ aviso.titulo }}</strong>
        <p v-if="aviso.detalle">{{ aviso.detalle }}</p>
        <p v-if="aviso.traceId" class="pista">Referencia: <code>{{ aviso.traceId }}</code></p>
      </div>

      <form class="grupo" aria-labelledby="titulo-datos" @submit.prevent="onGuardar">
        <h2 id="titulo-datos">Datos</h2>
        <BaseField label="Nombre" :error="errores.displayName" required>
          <template #default="{ id, describedBy, invalid }">
            <BaseInput
              :id="id"
              v-model="datos.displayName"
              :described-by="describedBy"
              :invalid="invalid"
              :disabled="!puedeEscribir"
              data-campo="displayName"
            />
          </template>
        </BaseField>
        <BaseField label="Correo" :error="errores.email" required>
          <template #default="{ id, describedBy, invalid }">
            <BaseInput
              :id="id"
              v-model="datos.email"
              type="email"
              :described-by="describedBy"
              :invalid="invalid"
              :disabled="!puedeEscribir"
              data-campo="email"
            />
          </template>
        </BaseField>
        <div v-if="puedeEscribir" class="barra">
          <BaseButton type="submit" :loading="guardando" :disabled="!pendientes">Guardar</BaseButton>
          <BaseButton variant="ghost" :disabled="!pendientes || guardando" @click="descartar">Descartar cambios</BaseButton>
        </div>
        <p v-else class="pista" data-aviso="solo-lectura">Solo lectura: para editar hace falta el permiso identity.user.write.</p>
      </form>

      <fieldset class="grupo" :disabled="!puedeAsignar" data-grupo="roles">
        <legend>Roles</legend>
        <p v-if="!puedeAsignar" class="pista">Para cambiar roles hace falta el permiso identity.role.assign.</p>
        <p v-if="rolesVisibles.length === 0" class="pista">Sin roles: puede entrar, pero no ve ninguna seccion.</p>
        <label v-for="rol in rolesVisibles" :key="rol.key" class="rol">
          <input
            type="checkbox"
            :checked="cuenta.roles.includes(rol.key)"
            :disabled="!puedeAsignar || cambiandoRol !== null"
            :data-rol="rol.key"
            @change="alternarRol(rol.key, $event.target as HTMLInputElement)"
          >
          <span>{{ rol.name }}</span>
          <code>{{ rol.key }}</code>
        </label>
      </fieldset>

      <div v-if="puedeEscribir" class="grupo" data-grupo="acceso">
        <h2>Acceso</h2>
        <template v-if="cuenta.enabled">
          <p class="pista">Deshabilitarla la saca al instante: se cierran todas sus sesiones.</p>
          <div class="barra">
            <BaseButton variant="danger" :loading="cambiandoAcceso" @click="confirmarBaja = true">Deshabilitar cuenta</BaseButton>
          </div>
        </template>
        <template v-else>
          <p class="pista">Esta cuenta no puede entrar. Al habilitarla, vuelve a entrar con la contrasena que tenia.</p>
          <div class="barra">
            <BaseButton :loading="cambiandoAcceso" @click="cambiarAcceso(true)">Habilitar cuenta</BaseButton>
          </div>
        </template>
      </div>

      <BaseDialog
        v-model:open="confirmarBaja"
        title="¿Deshabilitar esta cuenta?"
        :description="
          esPropia
            ? 'Es tu propia cuenta: vas a salir del dashboard en cuanto confirmes, y otra persona tendra que habilitarte.'
            : `${cuenta.displayName} deja de poder entrar y se cierran todas sus sesiones.`
        "
      >
        <template #acciones>
          <BaseButton variant="secondary" @click="confirmarBaja = false">Cancelar</BaseButton>
          <BaseButton variant="danger" @click="cambiarAcceso(false)">Deshabilitar</BaseButton>
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
h2,
legend {
  margin: 0;
  padding: 0;
  font-size: var(--text-base);
  font-weight: 600;
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
  max-width: 40rem;
}
.cabecera {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  align-items: flex-start;
  justify-content: space-between;
}
.insignias,
.barra {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}
.grupo {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin: 0;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}
.rol {
  display: flex;
  gap: var(--space-2);
  align-items: center;
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
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
</style>
