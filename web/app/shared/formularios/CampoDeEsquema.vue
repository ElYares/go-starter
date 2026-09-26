<script setup lang="ts">
// Un campo del editor, armado desde el JSON Schema del catalogo. Es recursivo:
// un objeto pinta un campo por propiedad y una lista un campo por elemento.
//
// Entiende lo que usa el catalogo del starter —texto, objeto, lista— y nada
// mas. Un tipo que no entiende no se pierde: se muestra su JSON y se conserva
// tal cual al guardar.
//
// `ruta` es la misma con la que el api nombra el campo en un 400
// (`blocks[2].props.items[0].title`), asi que el error llega solo al campo que
// lo tiene.
import { computed, ref, watch } from 'vue'
import { BaseButton, BaseField, BaseInput, BaseTextarea } from '~/shared/ui'
import type { Schemas } from '~/shared/api/generated'
import CampoDeMedio from './CampoDeMedio.vue'
import { esColor, FORMATO_COLOR, FORMATO_MEDIO, mover, valorInicial, type Esquema } from './esquema'

defineOptions({ name: 'CampoDeEsquema' })

const props = withDefaults(
  defineProps<{
    esquema: Esquema
    etiqueta: string
    ruta: string
    errores: Record<string, string>
    requerido?: boolean
    deshabilitado?: boolean
    /** La raiz de las props de un bloque: sus campos van sin recuadro propio. */
    raiz?: boolean
    /**
     * Sube una imagen para los campos `format: media-id`. Sin ella, esos campos
     * muestran la imagen y no se pueden cambiar.
     */
    subirMedio?: (archivo: File) => Promise<Schemas['Medio']>
  }>(),
  { requerido: false, deshabilitado: false, raiz: false },
)

const valor = defineModel<unknown>()

const error = computed(() => props.errores[props.ruta])

// Texto largo desde 300 caracteres: es la bajada del hero y el texto de un
// punto, que no caben en una linea. El umbral es del editor, no del esquema.
// Con `pattern` no: un enlace admite 500 caracteres y sigue siendo una linea.
const esLargo = computed(() => (props.esquema.maxLength ?? 0) >= 300 && !props.esquema.pattern)

const texto = computed({
  get: () => (typeof valor.value === 'string' ? valor.value : ''),
  // Un opcional vaciado se quita: "sin bajada" no es una bajada "".
  set: (v: string) => {
    valor.value = !props.requerido && v === '' ? undefined : v
  },
})

// La muestra enseña el ultimo color valido: mientras se escribe "#2f6d" no hay
// color que pintar, y un <input type="color"> con un valor invalido se pone
// negro, que parece un color elegido. El hex escrito manda; la muestra lo sigue.
const ultimoColor = ref('#000000')
watch(
  texto,
  (v) => {
    if (esColor(v)) ultimoColor.value = v
  },
  { immediate: true },
)

const objeto = computed(() =>
  valor.value && typeof valor.value === 'object' && !Array.isArray(valor.value)
    ? (valor.value as Record<string, unknown>)
    : undefined,
)

function asignar(clave: string, v: unknown) {
  const copia = { ...(objeto.value ?? {}) }
  if (v === undefined) delete copia[clave]
  else copia[clave] = v
  valor.value = copia
}

const lista = computed(() => (Array.isArray(valor.value) ? valor.value : []))
const minimo = computed(() => props.esquema.minItems ?? 0)
const maximo = computed(() => props.esquema.maxItems ?? Number.POSITIVE_INFINITY)

function cambiarElemento(i: number, v: unknown) {
  const copia = [...lista.value]
  copia[i] = v
  valor.value = copia
}

function agregarElemento() {
  valor.value = [...lista.value, props.esquema.items ? valorInicial(props.esquema.items) : null]
}

function quitarElemento(i: number) {
  valor.value = lista.value.filter((_, j) => j !== i)
}

function moverElemento(i: number, delta: -1 | 1) {
  valor.value = mover(lista.value, i, delta)
}

const etiquetaDeElemento = (i: number) => `${props.esquema.items?.title ?? 'Elemento'} ${i + 1}`
</script>

<template>
  <CampoDeMedio
    v-if="esquema.type === 'string' && esquema.format === FORMATO_MEDIO"
    :model-value="typeof valor === 'string' ? valor : undefined"
    @update:model-value="valor = $event"
    :etiqueta="etiqueta"
    :ruta="ruta"
    :descripcion="esquema.description"
    :error="error"
    :requerido="requerido"
    :deshabilitado="deshabilitado"
    :subir="subirMedio"
  />

  <BaseField
    v-else-if="esquema.type === 'string' && esquema.format === FORMATO_COLOR"
    :label="etiqueta"
    :hint="esquema.description"
    :error="error"
    :required="requerido"
  >
    <template #default="{ id, describedBy, invalid }">
      <div class="color">
        <!-- La paleta del sistema, sobre la muestra. El nombre accesible lo
             lleva aparte: la etiqueta del campo es del hex, que es lo que se
             guarda y lo que marca un 400. -->
        <input
          type="color"
          class="muestra"
          :value="ultimoColor"
          :aria-label="`${etiqueta}: elegir en la paleta`"
          :disabled="deshabilitado"
          @input="texto = ($event.target as HTMLInputElement).value"
        />
        <BaseInput
          :id="id"
          v-model="texto"
          :described-by="describedBy"
          :invalid="invalid"
          :disabled="deshabilitado"
          :data-campo="ruta"
          placeholder="#rrggbb"
        />
      </div>
    </template>
  </BaseField>

  <BaseField
    v-else-if="esquema.type === 'string'"
    :label="etiqueta"
    :hint="esquema.description"
    :error="error"
    :required="requerido"
  >
    <template #default="{ id, describedBy, invalid }">
      <BaseTextarea
        v-if="esLargo"
        :id="id"
        v-model="texto"
        :described-by="describedBy"
        :invalid="invalid"
        :disabled="deshabilitado"
        :data-campo="ruta"
      />
      <BaseInput
        v-else
        :id="id"
        v-model="texto"
        :described-by="describedBy"
        :invalid="invalid"
        :disabled="deshabilitado"
        :data-campo="ruta"
      />
    </template>
  </BaseField>

  <div v-else-if="esquema.type === 'object' && raiz" class="campos">
    <CampoDeEsquema
      v-for="(prop, clave) in esquema.properties"
      :key="clave"
      :esquema="prop"
      :etiqueta="prop.title ?? String(clave)"
      :ruta="`${ruta}.${clave}`"
      :errores="errores"
      :requerido="esquema.required?.includes(String(clave))"
      :deshabilitado="deshabilitado"
      :subir-medio="subirMedio"
      :model-value="objeto?.[clave]"
      @update:model-value="asignar(String(clave), $event)"
    />
    <p v-if="error" class="error" role="alert">{{ error }}</p>
  </div>

  <fieldset v-else-if="esquema.type === 'object'" class="grupo" :data-campo="ruta">
    <legend>{{ etiqueta }}</legend>
    <p v-if="esquema.description" class="pista">{{ esquema.description }}</p>

    <template v-if="objeto || requerido">
      <div class="campos">
        <CampoDeEsquema
          v-for="(prop, clave) in esquema.properties"
          :key="clave"
          :esquema="prop"
          :etiqueta="prop.title ?? String(clave)"
          :ruta="`${ruta}.${clave}`"
          :errores="errores"
          :requerido="esquema.required?.includes(String(clave))"
          :deshabilitado="deshabilitado"
          :subir-medio="subirMedio"
          :model-value="objeto?.[clave]"
          @update:model-value="asignar(String(clave), $event)"
        />
      </div>
      <BaseButton
        v-if="!requerido && !deshabilitado"
        variant="ghost"
        size="sm"
        @click="valor = undefined"
      >
        Quitar {{ etiqueta.toLowerCase() }}
      </BaseButton>
    </template>
    <BaseButton
      v-else-if="!deshabilitado"
      variant="secondary"
      size="sm"
      @click="valor = valorInicial(esquema)"
    >
      Agregar {{ etiqueta.toLowerCase() }}
    </BaseButton>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
  </fieldset>

  <fieldset v-else-if="esquema.type === 'array'" class="grupo" :data-campo="ruta">
    <legend>{{ etiqueta }}</legend>
    <ol class="elementos">
      <li v-for="(elemento, i) in lista" :key="i" class="elemento">
        <div class="acciones-elemento">
          <BaseButton
            variant="ghost"
            size="sm"
            :disabled="deshabilitado || i === 0"
            :aria-label="`Subir ${etiquetaDeElemento(i)}`"
            @click="moverElemento(i, -1)"
          >
            Subir
          </BaseButton>
          <BaseButton
            variant="ghost"
            size="sm"
            :disabled="deshabilitado || i === lista.length - 1"
            :aria-label="`Bajar ${etiquetaDeElemento(i)}`"
            @click="moverElemento(i, 1)"
          >
            Bajar
          </BaseButton>
          <BaseButton
            variant="ghost"
            size="sm"
            :disabled="deshabilitado || lista.length <= minimo"
            :aria-label="`Quitar ${etiquetaDeElemento(i)}`"
            @click="quitarElemento(i)"
          >
            Quitar
          </BaseButton>
        </div>
        <CampoDeEsquema
          v-if="esquema.items"
          :esquema="esquema.items"
          :etiqueta="etiquetaDeElemento(i)"
          :ruta="`${ruta}[${i}]`"
          :errores="errores"
          requerido
          :deshabilitado="deshabilitado"
          :subir-medio="subirMedio"
          :model-value="elemento"
          @update:model-value="cambiarElemento(i, $event)"
        />
      </li>
    </ol>
    <BaseButton
      v-if="!deshabilitado"
      variant="secondary"
      size="sm"
      :disabled="lista.length >= maximo"
      @click="agregarElemento"
    >
      Agregar {{ (esquema.items?.title ?? 'elemento').toLowerCase() }}
    </BaseButton>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
  </fieldset>

  <!-- Lo que el editor no sabe pintar se ve y se conserva: al guardar viaja
       tal cual estaba. -->
  <div v-else class="grupo" :data-campo="ruta">
    <p class="etiqueta">{{ etiqueta }}</p>
    <p class="pista">Este campo no se puede editar desde aqui. Se conserva como esta.</p>
    <pre>{{ JSON.stringify(valor, null, 2) }}</pre>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.color {
  display: flex;
  align-items: stretch;
  gap: var(--space-2);
}
.muestra {
  flex: none;
  width: 2.75rem;
  min-height: 2.5rem;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: none;
  cursor: pointer;
}
.muestra:focus-visible {
  outline: var(--focus-ring) solid var(--color-accent-strong);
  outline-offset: var(--focus-ring);
}
.muestra:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
.muestra::-webkit-color-swatch-wrapper {
  padding: var(--space-1);
}
.muestra::-webkit-color-swatch {
  border: none;
  border-radius: var(--radius-sm);
}
.muestra::-moz-color-swatch {
  border: none;
  border-radius: var(--radius-sm);
}

.campos {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.grupo {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin: 0;
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}
legend,
.etiqueta {
  margin: 0;
  padding: 0 var(--space-1);
  font-size: var(--text-sm);
  font-weight: 600;
}
.elementos {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin: 0;
  padding: 0;
  list-style: none;
}
.elemento {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.acciones-elemento {
  display: flex;
  gap: var(--space-1);
  justify-content: flex-end;
}
.pista {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.error {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-danger);
}
pre {
  margin: 0;
  padding: var(--space-2);
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: var(--text-sm);
  background: var(--color-surface-hover);
  border-radius: var(--radius-sm);
}
</style>
