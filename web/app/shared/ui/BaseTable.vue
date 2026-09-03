<script setup lang="ts">
export interface ColumnaTabla {
  key: string
  label: string
  align?: 'start' | 'end'
}

// Las filas van sin tipar a proposito: una tabla generica no puede saber que
// forma tiene el recurso de cada modulo, y forzar un tipo aqui obligaria a cada
// fork a pelearse con genericos para pintar cuatro celdas.
withDefaults(
  defineProps<{
    columns: ColumnaTabla[]
    rows: Array<Record<string, unknown>>
    rowKey?: string
    caption?: string
  }>(),
  { rowKey: 'id' },
)
</script>

<template>
  <!-- El contenedor con scroll horizontal esta aqui y no en cada vista: una
       tabla de seis columnas en un movil desborda la pagina entera, y el
       sintoma es un desplazamiento lateral que nadie asocia a la tabla. -->
  <div class="marco">
    <table class="tabla">
      <caption v-if="caption" class="leyenda">{{ caption }}</caption>
      <thead>
        <tr>
          <th
            v-for="columna in columns"
            :key="columna.key"
            scope="col"
            :class="columna.align === 'end' ? 'al-final' : undefined"
          >
            {{ columna.label }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="rows.length === 0">
          <!-- El vacio dentro de la propia tabla, y no como hermano: si se pone
               fuera, la cabecera queda flotando sobre la nada. -->
          <td :colspan="columns.length" class="vacio">
            <slot name="vacio">Sin resultados.</slot>
          </td>
        </tr>
        <tr v-for="(fila, i) in rows" v-else :key="String(fila[rowKey] ?? i)">
          <td
            v-for="columna in columns"
            :key="columna.key"
            :class="columna.align === 'end' ? 'al-final' : undefined"
          >
            <!-- Un slot por columna deja al fork pintar una pildora, un enlace o
                 una fecha formateada sin tocar este componente. -->
            <slot :name="`celda-${columna.key}`" :fila="fila" :valor="fila[columna.key]">
              {{ fila[columna.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.marco {
  overflow-x: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.tabla {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--text-sm);
}

.leyenda {
  padding: var(--space-3);
  color: var(--color-text-muted);
  text-align: start;
}

th,
td {
  padding: var(--space-3);
  text-align: start;
  border-bottom: 1px solid var(--color-border);
}

th {
  background: var(--color-surface-hover);
  color: var(--color-text);
  font-weight: 600;
}

tbody tr:last-child td {
  border-bottom: none;
}

tbody tr:hover td {
  background: var(--color-surface-hover);
}

.al-final {
  text-align: end;
}

.vacio {
  padding: var(--space-8) var(--space-4);
  color: var(--color-text-muted);
  text-align: center;
}
</style>
