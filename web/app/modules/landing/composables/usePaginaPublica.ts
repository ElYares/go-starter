import type { Schemas } from '~/shared/api/generated'
import { useApiFetch } from '~/shared/api/useApiFetch'
import { codigoDeFallo, statusRespondido } from '../pagina'

/**
 * Pide la version PUBLICADA de una pagina. En SSR sale por la red interna del
 * compose (`apiInternal`), que es lo que resuelve `useApiFetch`.
 *
 * Si la API no la entrega, lanza el error de Nuxt con el codigo que corresponde
 * y `fatal`, para que el servidor responda con ese codigo real y no con un 200
 * que pinta la pantalla de error. Ver `codigoDeFallo`.
 */
export async function usePaginaPublica(slug: string) {
  const { data, error } = await useApiFetch<Schemas['PaginaPublica']>(`/public/pages/${slug}`)

  if (error.value || !data.value) {
    const codigo = codigoDeFallo(statusRespondido(error.value))

    // Ni un 404 ni un 429 son un fallo del sitio: el log se queda para lo que si.
    if (codigo !== 404 && codigo !== 429) {
      // El detalle se queda en el log del servidor; al visitante le llega el
      // codigo y un mensaje generico.
      console.error(`[landing] la API no entrego la pagina "${slug}": ${error.value?.message ?? 'sin datos'}`)
    }

    throw createError({
      statusCode: codigo,
      statusMessage:
        codigo === 404 ? 'Pagina no encontrada' : codigo === 429 ? 'Demasiadas visitas seguidas' : 'El sitio no esta disponible',
      data: { landing: true },
      fatal: true,
    })
  }

  return data as Ref<Schemas['PaginaPublica']>
}
