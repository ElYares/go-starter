// @vitest-environment happy-dom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BaseTextarea from './BaseTextarea.vue'

describe('BaseTextarea', () => {
  it('va y viene con v-model, saltos de linea incluidos', async () => {
    const area = mount(BaseTextarea, { props: { modelValue: 'uno' } })
    expect(area.element.value).toBe('uno')

    await area.setValue('uno\n\ndos')
    expect(area.emitted('update:modelValue')).toEqual([['uno\n\ndos']])
  })

  it('invalido se anuncia, y acepta lo que le pasa BaseField', () => {
    const area = mount(BaseTextarea, { props: { invalid: true, id: 'x', describedBy: 'x-error' } })
    expect(area.attributes('aria-invalid')).toBe('true')
    expect(area.attributes('id')).toBe('x')
    expect(area.attributes('aria-describedby')).toBe('x-error')
  })
})
