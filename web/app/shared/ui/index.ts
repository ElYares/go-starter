// El unico sitio por el que una vista entra a los primitivos. Importar de aqui y
// no del archivo suelto es lo que permite mover, renombrar o partir un Base* sin
// tocar quien lo usa.
export { default as BaseBadge } from './BaseBadge.vue'
export { default as BaseButton } from './BaseButton.vue'
export { default as BaseDialog } from './BaseDialog.vue'
export { default as BaseEmptyState } from './BaseEmptyState.vue'
export { default as BaseField } from './BaseField.vue'
export { default as BaseInput } from './BaseInput.vue'
export { default as BaseSelect } from './BaseSelect.vue'
export { default as BaseTable } from './BaseTable.vue'
export { default as BaseToast } from './BaseToast.vue'

export type { ColumnaTabla } from './BaseTable.vue'
export type { OpcionSelect } from './BaseSelect.vue'
