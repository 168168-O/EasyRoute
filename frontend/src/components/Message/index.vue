<script setup lang="ts">
import bull from '@/assets/bull-crystal.webp'
import i18n from '@/lang'

export type MessageIcon = 'info' | 'warn' | 'error' | 'success'

interface Props {
  icon?: MessageIcon
  content: string
}

const props = withDefaults(defineProps<Props>(), {
  icon: 'info',
})

defineEmits(['close'])

const { t } = i18n.global
</script>

<template>
  <Transition name="slide-down" appear>
    <div
      class="gui-message flex items-center p-8 pl-16 rounded-8 my-4 shadow"
      role="status"
      :data-kind="props.icon"
    >
      <img :src="bull" alt="" draggable="false" class="msg-bull shrink-0" />
      <div class="text-14 pl-12 break-all">{{ t(content) }}</div>
      <Button
        icon="close"
        :icon-size="10"
        type="text"
        size="small"
        class="close px-4 invisible"
        @click="$emit('close')"
      />
    </div>
  </Transition>
</template>

<style lang="less" scoped>
.gui-message {
  background: var(--toast-bg);
  &:hover {
    .close {
      visibility: visible;
    }
  }
}

.msg-bull {
  width: 16px;
  height: 16px;
  object-fit: contain;
  border-radius: 4px;
}
</style>
