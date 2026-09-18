<script setup lang="ts">
// Un campo `format: media-id`: el id de una imagen subida. Se edita subiendo un
// archivo, no escribiendo un uuid.
//
// Sube en cuanto se elige el archivo, y solo entonces cambia el valor: si la
// subida falla, el campo conserva la imagen que tenia. Guardar es aparte, en el
// formulario: una imagen subida y no guardada queda como medio huerfano, y eso
// esta aceptado mientras no haya biblioteca (Decision 024).
//
// Sin `subir` —quien mira no tiene permiso, o el fork no tiene media— se ve la
// imagen y no se puede cambiar.
import { computed, ref } from 'vue'
import { BaseButton, BaseField } from '~/shared/ui'
import { ApiError } from '~/shared/api/errors'
import type { Schemas } from '~/shared/api/generated'
import { MAX_BYTES_MEDIO, urlDeMedio } from './esquema'

const props = withDefaults(
  defineProps<{
    etiqueta: string
    ruta: string
    descripcion?: string
    /** El error del servidor para este campo, por ejemplo `value.logo:unknown`. */
    error?: string
    requerido?: boolean
    deshabilitado?: boolean
    subir?: (archivo: File) => Promise<Schemas['Medio']>
  }>(),
  { requerido: false, deshabilitado: false },
)

const valor = defineModel<string | undefined>()

const subiendo = ref(false)
const falloDeSubida = ref<string>()

const vistaPrevia = computed(() => (valor.value ? urlDeMedio(valor.value) : undefined))
const error = computed(() => falloDeSubida.value ?? props.error)
const puedeCambiar = computed(() => !!props.subir && !props.deshabilitado)

async function elegir(evento: Event) {
  const entrada = evento.target as HTMLInputElement
  const archivo = entrada.files?.[0]
  // Se limpia para que elegir el mismo archivo otra vez vuelva a disparar.
  entrada.value = ''
  if (!archivo || !props.subir) return

  falloDeSubida.value = undefined
  // El tope se comprueba antes de mandar nada: por encima de cierto tamano, el
  // 413 del servidor llega como error de red y no se podria explicar.
  if (archivo.size > MAX_BYTES_MEDIO) {
    falloDeSubida.value = 'El archivo pasa de 5 MB'
    return
  }

  subiendo.value = true
  try {
    const medio = await props.subir(archivo)
    valor.value = medio.id
  } catch (causa) {
    if (!(causa instanceof ApiError)) throw causa
    falloDeSubida.value = mensajeDe(causa)
  } finally {
    subiendo.value = false
  }
}

function mensajeDe(causa: ApiError): string {
  if (causa.status === 413) return 'El archivo pasa de 5 MB'
  if (causa.status === 403) return 'No tienes permiso para subir imagenes'
  if (causa.unavailable) return 'El servidor no responde. Vuelve a intentarlo'
  return causa.errors?.[0]?.message ?? causa.message
}
</script>

<template>
  <BaseField :label="etiqueta" :hint="descripcion" :error="error" :required="requerido">
    <template #default="{ id, describedBy, invalid }">
      <div class="medio" :data-campo="ruta" :aria-busy="subiendo || undefined">
        <img v-if="vistaPrevia" :src="vistaPrevia" :alt="`${etiqueta} actual`" class="vista" />
        <p v-else class="vacio">Sin imagen</p>

        <div v-if="puedeCambiar" class="acciones">
          <input
            :id="id"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            class="archivo"
            :aria-describedby="describedBy"
            :aria-invalid="invalid || undefined"
            :disabled="subiendo"
            @change="elegir"
          />
          <BaseButton
            v-if="valor && !requerido"
            variant="ghost"
            size="sm"
            :disabled="subiendo"
            @click="valor = undefined"
          >
            Quitar {{ etiqueta.toLowerCase() }}
          </BaseButton>
        </div>
        <p v-if="subiendo" class="pista" role="status">Subiendo…</p>
      </div>
    </template>
  </BaseField>
</template>

<style scoped>
.medio {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.vista {
  max-width: 12rem;
  max-height: 6rem;
  object-fit: contain;
  align-self: flex-start;
  padding: var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface-hover);
}
.vacio,
.pista {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
.acciones {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}
.archivo {
  font-size: var(--text-sm);
}
</style>
