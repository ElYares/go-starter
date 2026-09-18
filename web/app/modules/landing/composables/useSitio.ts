import type { Schemas } from '~/shared/api/generated'
import { useApiFetch } from '~/shared/api/useApiFetch'
import { statusRespondido } from '../pagina'
import { sitioDe, type Sitio } from '../sitio'

/**
 * Pide la configuracion publica del sitio para pintar su marco. En SSR sale por
 * la red interna, como la pagina.
 *
 * **Si falla, la pagina se sirve igual, sin cabecera ni pie** (HU-013,
 * criterio 6). El marco es secundario: una caida de la configuracion no tumba
 * el contenido con un 503. El fallo queda en el log de web, que es donde se ve.
 */
export async function useSitio(): Promise<Ref<Sitio | null>> {
  const { data, error } = await useApiFetch<Schemas['SettingsMap']>('/public/settings')

  if (error.value) {
    const status = statusRespondido(error.value)
    console.error(
      `[landing] no llego la configuracion del sitio (${status ?? 'sin respuesta'}): se sirve sin cabecera ni pie`,
    )
  }

  return computed(() => (error.value ? null : sitioDe(data.value)))
}
