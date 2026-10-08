<script setup lang="ts">
import { nextTick } from 'vue'

import bull from '@/assets/bull-crystal.webp'
import i18n from '@/lang'

interface Props {
  size?: 'default' | 'small'
  border?: 'default' | 'square'
  label?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 'default',
  border: 'default',
  label: '',
  disabled: false,
})

const model = defineModel<boolean>({ default: false })

const emits = defineEmits<{
  (e: 'change', val: boolean): void
}>()

const { t } = i18n.global

const toggle = () => {
  if (props.disabled) return
  model.value = !model.value
  nextTick(() => emits('change', model.value))
}
</script>

<template>
  <div
    v-tips.slow="label"
    :aria-checked="model"
    :aria-disabled="disabled || undefined"
    :class="[
      size,
      border,
      model ? 'on' : 'off',
      border === 'square' ? 'rounded-4' : 'rounded-full',
      { 'cursor-not-allowed': disabled },
    ]"
    class="gui-switch relative cursor-pointer h-24 inline-flex items-center text-12 duration-200"
    role="switch"
    @click="toggle"
  >
    <img :src="bull" alt="" draggable="false" class="dot" />

    <div v-if="$slots.default || label" class="slot">
      <span v-if="label">{{ t(label) }}</span>
      <slot v-if="$slots.default"></slot>
    </div>
  </div>
</template>

<style lang="less" scoped>
.gui-switch {
  box-sizing: border-box;
  gap: 6px;
  min-width: 52px;
  padding: 2px 8px 2px 3px;
  flex: none;

  .dot {
    position: relative;
    top: auto;
    left: auto;
    flex: none;
    width: 20px;
    height: 20px;
    border-radius: 999px;
    object-fit: cover;
    pointer-events: none;
  }

  .slot {
    white-space: nowrap;
    line-height: 1.2;
  }

  &:not(:has(.slot)) {
    width: 52px;
    padding: 2px;
    &.on {
      justify-content: flex-end;
    }
  }
}

.on:has(.slot) {
  flex-direction: row-reverse;
  padding: 2px 3px 2px 8px;
}

.small {
  .dot {
    width: 18px;
    height: 18px;
  }
  &:not(:has(.slot)) {
    width: 46px;
  }
}
</style>
