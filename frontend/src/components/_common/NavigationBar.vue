<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import bull from '@/assets/bull-crystal.webp'
import rawRoutes from '@/router/routes'
import { useAppSettingsStore } from '@/stores'

const { t } = useI18n()
const appSettings = useAppSettingsStore()

const SIMPLE_NAV = ['Overview', 'AppRouting', 'Subscriptions', 'Settings', 'Translate']

const routes = computed(() => {
  const visible = rawRoutes.filter(
    (r) =>
      r.meta?.hidden === false ||
      (!r.meta?.hidden && appSettings.app.pages.includes(r.name! as string)),
  )
  if (appSettings.app.simpleMode === false) return visible
  return visible.filter((route) => SIMPLE_NAV.includes(String(route.name)))
})
</script>

<template>
  <div class="main-nav flex items-center justify-center">
    <div
      v-for="r in routes"
      :key="r.path"
      :class="['main-nav__item', `main-nav__item--${String(r.name).toLowerCase()}`]"
    >
      <RouterLink v-slot="{ navigate, isActive }" :to="r.path" custom>
        <Button
          :class="{ 'is-active': isActive }"
          class="main-nav__button"
          :type="isActive ? 'link' : 'text'"
          @click="navigate"
        >
          <img :src="bull" alt="" draggable="false" class="nav-ico" />
          {{ (r.meta && t(r.meta.name)) || r.name }}
        </Button>
      </RouterLink>
    </div>
  </div>
</template>
