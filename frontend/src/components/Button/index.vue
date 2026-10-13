<script setup lang="ts">
import { computed, useAttrs } from 'vue'

import bull from '@/assets/bull-crystal.webp'
import { type IconName } from '@/components/Icon/icons'

interface Props {
  type?: 'primary' | 'normal' | 'link' | 'text'
  size?: 'default' | 'small' | 'large'
  iconSize?: number
  iconColor?: string
  icon?: IconName
  loading?: boolean
  disabled?: boolean
  role?: string
}

const props = withDefaults(defineProps<Props>(), {
  type: 'normal',
  size: 'default',
  iconSize: undefined,
  iconColor: undefined,
  icon: undefined,
  loading: false,
  disabled: false,
  role: 'button',
})

const attrs = useAttrs()

/** Window chrome keeps close / minimise glyphs. Every other button icon is the bull head. */
const showBull = computed(() => {
  const raw = attrs.class
  const text = Array.isArray(raw) ? raw.map((part) => String(part)).join(' ') : String(raw ?? '')
  return !text.split(/\s+/).includes('window-control')
})

const bullSize = computed(() => props.iconSize ?? (props.size === 'small' ? 14 : 16))
</script>

<template>
  <div
    :role="role"
    :aria-disabled="disabled || loading ? true : undefined"
    :class="[type, size, { 'pointer-events-none': disabled || loading }]"
    class="gui-button inline-flex items-center justify-center text-center align-middle rounded-6 text-14 text-nowrap cursor-pointer px-12 py-6 duration-200"
  >
    <Icon
      v-if="loading"
      color="currentColor"
      :size="size === 'small' ? 14 : 16"
      icon="loading"
      class="rotation"
    />
    <template v-else>
      <Icon
        v-if="disabled"
        color="currentColor"
        icon="forbidden"
        class="pointer-events-none shrink-0 mr-4"
      />
      <img
        v-if="icon && showBull"
        :src="bull"
        alt=""
        draggable="false"
        class="bull-ico"
        :class="$slots.default ? 'mr-4' : ''"
        :style="{ width: bullSize + 'px', height: bullSize + 'px' }"
      />
      <Icon
        v-else-if="icon"
        :icon="icon"
        :size="iconSize"
        :color="iconColor || 'currentColor'"
        :class="$slots.default ? 'mr-4' : ''"
      />
    </template>
    <slot></slot>
  </div>
</template>

<style lang="less" scoped>
.normal {
  color: var(--btn-normal-color);
  background-color: var(--btn-normal-bg);
  &:hover {
    color: var(--btn-normal-hover-color);
    border-color: var(--btn-normal-hover-border-color);
  }
  &:active {
    color: var(--btn-normal-active-color);
    border-color: var(--btn-normal-active-border-color);
  }
}

.primary {
  color: var(--btn-primary-color);
  background-color: var(--btn-primary-bg);
  border: none;
  &:hover {
    background-color: var(--btn-primary-hover-bg);
  }
  &:active {
    background-color: var(--btn-primary-active-bg);
  }
}

.link {
  color: var(--btn-link-color);
  background-color: var(--btn-link-bg);
  border: none;
  &:hover {
    color: var(--btn-link-hover-color);
  }
  &:active {
    color: var(--btn-link-active-color);
  }
}

.text {
  color: var(--btn-text-color);
  background-color: var(--btn-text-bg);
  border: none;
  &:hover {
    color: var(--btn-text-hover-color);
    background-color: var(--btn-text-hover-bg);
  }
  &:active {
    color: var(--btn-text-active-color);
    background-color: var(--btn-text-active-bg);
  }
}

.small {
  padding: 4px 8px;
  font-size: 12px;
}
.large {
  padding: 8px 12px;
  font-size: 16px;
}

.bull-ico {
  flex: none;
  object-fit: contain;
  pointer-events: none;
  border-radius: 4px;
}
</style>
