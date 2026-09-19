// Sin el secreto del SSR, el web no arranca, igual que el api sin el suyo
// (HU-010). Arrancar sin el serviria la landing, pero todas sus visitas
// contarian contra la IP del contenedor en el limite del api: la falla
// apareceria como un 429 general en el primer pico de trafico, lejos de su
// causa. Mejor que el proceso muera ahora y diga que falta.
export default defineNitroPlugin(() => {
  if (!useRuntimeConfig().apiSsrSecret) {
    throw new Error('Falta NUXT_API_SSR_SECRET (en el compose, SSR_SECRET). No tiene valor por omision a proposito')
  }
})
