import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  getProxies,
  getConfigs,
  setConfigs,
  onLogs,
  onMemory,
  onConnections,
  onTraffic,
  initWebsocket,
  destroyWebsocket,
  probeApiAvailability,
} from '@/api/kernel'
import { ExitApp, ProcessInfo, KillProcess, ExecBackground, ReadFile, RemoveFile, CleanupNetworkAfterStop } from '@/bridge'
import {
  CoreConfigFilePath,
  CoreLogFilePath,
  CorePidFilePath,
  CoreWorkingDirectory,
} from '@/constant/kernel'
import { DefaultInboundMixed } from '@/constant/profile'
import { Branch, OS } from '@/enums/app'
import { Inbound, RulesetType, TunStack } from '@/enums/kernel'
import {
  useAppSettingsStore,
  useProfilesStore,
  useLogsStore,
  useEnvStore,
  usePluginsStore,
  useSubscribesStore,
  useRulesetsStore,
} from '@/stores'
import {
  generateConfigFile,
  updateTrayAndMenus,
  getKernelFileName,
  normalizeProxyHost,
  restoreProfile,
  deepClone,
  message,
  confirm,
  RunWithPowerShell,
  getKernelRuntimeArgs,
  getKernelRuntimeEnv,
  eventBus,
  sleep,
} from '@/utils'

import type { CoreApiConfig, CoreApiProxy } from '@/types/kernel'

export type ProxyType = 'mixed' | 'http' | 'socks'
export type ProxyEndpoint = {
  schema: 'http' | 'socks5'
  host: string
  port: number
  username: string
  password: string
  proxyType: ProxyType
}

export const useKernelApiStore = defineStore('kernelApi', () => {
  const envStore = useEnvStore()
  const logsStore = useLogsStore()
  const pluginsStore = usePluginsStore()
  const profilesStore = useProfilesStore()
  const subscribesStore = useSubscribesStore()
  const rulesetsStore = useRulesetsStore()
  const appSettingsStore = useAppSettingsStore()

  /** RESTful API */
  const config = ref<CoreApiConfig>({
    port: 0,
    'mixed-port': 0,
    'socks-port': 0,
    'interface-name': '',
    'allow-lan': false,
    mode: '',
    tun: {
      enable: false,
      stack: '',
      device: '',
    },
  })

  let runtimeProfile: App.Profile | undefined

  const proxies = ref<Record<string, CoreApiProxy>>({})

  const refreshConfig = async () => {
    const _config = await getConfigs()

    config.value = {
      ..._config,
      tun: config.value.tun,
    }

    if (!runtimeProfile) {
      const txt = await ReadFile(CoreConfigFilePath)
      runtimeProfile = restoreProfile(JSON.parse(txt))
      const profile = profilesStore.currentProfile
      if (profile) {
        const _profile = deepClone(profile)
        _profile.inbounds.forEach((inbound) => {
          const runtimeInbound = runtimeProfile?.inbounds.find((v) => v.tag === inbound.tag)
          if (runtimeInbound) {
            runtimeInbound.id = inbound.id
          } else {
            inbound.enable = false
            runtimeProfile?.inbounds.push(inbound)
          }
        })
        runtimeProfile.id = _profile.id
        runtimeProfile.outbounds = _profile.outbounds
        runtimeProfile.experimental = _profile.experimental
        runtimeProfile.dns = _profile.dns
        runtimeProfile.route = _profile.route
        runtimeProfile.mixin = _profile.mixin
        runtimeProfile.script = _profile.script
      }
    }

    const mixed = runtimeProfile.inbounds.find((v) => v.enable && v.mixed)
    const http = runtimeProfile.inbounds.find((v) => v.enable && v.http)
    const socks = runtimeProfile.inbounds.find((v) => v.enable && v.socks)
    const tun = runtimeProfile.inbounds.find((v) => v.tun)
    config.value['mixed-port'] = mixed?.mixed?.listen.listen_port || 0
    config.value['port'] = http?.http?.listen.listen_port || 0
    config.value['socks-port'] = socks?.socks?.listen.listen_port || 0
    config.value['allow-lan'] = [
      mixed?.mixed?.listen.listen,
      http?.http?.listen.listen,
      socks?.socks?.listen.listen,
    ].some((address) => address === '0.0.0.0' || address === '::')

    // The switch follows the saved choice. A generated config must not flip it back on.
    config.value.tun.enable = appSettingsStore.app.tunEnabled !== false && !!tun?.enable
    config.value.tun.device = tun?.tun?.interface_name || ''
    config.value.tun.stack = tun?.tun?.stack || ''
    config.value['interface-name'] = runtimeProfile.route.default_interface
  }

  const resetConfig = () => {
    config.value.port = 0
    config.value['socks-port'] = 0
    config.value['mixed-port'] = 0
    config.value['interface-name'] = ''
    config.value['allow-lan'] = false
    config.value.mode = ''
    // Keep the saved switch. A core restart must not paint TUN back on by itself.
    config.value.tun.enable = appSettingsStore.app.tunEnabled !== false
    config.value.tun.stack = ''
    config.value.tun.device = ''
  }

  const updateConfig = async (field: string, value: any) => {
    if (field === 'mode') {
      await setConfigs({ mode: value })
      await refreshConfig()
      return
    }

    const patchInbound = () => {
      if (!runtimeProfile) return
      const inbound = runtimeProfile.inbounds.find(
        (v) =>
          (v.type === Inbound.Mixed && v.mixed?.listen.listen_port) ||
          (v.type === Inbound.Http && v.http?.listen.listen_port) ||
          (v.type === Inbound.Socks && v.socks?.listen.listen_port),
      )
      if (!inbound) {
        throw 'home.overview.needPort'
      }
      inbound.enable = true
    }

    const patchInboundPort = (type: 'mixed' | 'socks' | 'http', port: number) => {
      if (!runtimeProfile) return
      let inbound = runtimeProfile.inbounds.find((v) => v.type === type)
      if (inbound) {
        inbound[type]!.listen.listen_port = port
      } else {
        const _type = DefaultInboundMixed()!
        _type.listen.listen_port = port
        inbound = {
          id: type + '-in',
          tag: type + '-in',
          type: type,
          enable: true,
          [type]: _type,
        }
        runtimeProfile.inbounds.push(inbound)
      }
      inbound.enable = port !== 0
    }

    const patchInboundAddress = (allowLan: boolean) => {
      if (!runtimeProfile) return
      runtimeProfile.inbounds.forEach((inbound) => {
        if (inbound.type === Inbound.Tun) return
        inbound[inbound.type]!.listen.listen = allowLan ? '0.0.0.0' : '127.0.0.1'
      })
    }

    const patchInboundTun = (options: {
      enable: boolean
      stack: string
      device: string
      interface_name: string
    }) => {
      if (!runtimeProfile) return
      const inbound = runtimeProfile.inbounds.find((v) => v.type === Inbound.Tun)
      if (!inbound) throw 'home.overview.needTun'
      options = { ...config.value.tun, ...options }
      inbound.enable = options.enable
      inbound.tun!.stack = (options.stack || TunStack.Mixed) as App.TunStack
      inbound.tun!.interface_name = options.device || ''
      if (options.interface_name) {
        runtimeProfile.route.default_interface = options.interface_name
      }
      runtimeProfile.route.auto_detect_interface = !options.interface_name
    }

    const fieldHandlerMap: Recordable<() => void> = {
      inbound: () => patchInbound(),
      http: () => patchInboundPort(Inbound.Http, value),
      socks: () => patchInboundPort(Inbound.Socks, value),
      mixed: () => patchInboundPort(Inbound.Mixed, value),
      'allow-lan': () => patchInboundAddress(value),
      tun: () => patchInboundTun(value),
      'tun-stack': () => patchInboundTun(value),
      'tun-device': () => patchInboundTun(value),
      'interface-name': () => patchInboundTun(value),
    }

    fieldHandlerMap[field]?.()

    const previousTun = appSettingsStore.app.tunEnabled !== false
    if (field === 'tun') {
      appSettingsStore.app.tunEnabled = !!value?.enable
    }

    try {
      await restartCore(undefined, true)
      await envStore.updateSystemProxyStatus()
    } catch (error) {
      if (field === 'tun') appSettingsStore.app.tunEnabled = previousTun
      throw error
    }
  }

  const refreshProviderProxies = async () => {
    const { proxies: b } = await getProxies()
    proxies.value = b
  }

  const { t } = useI18n()

  /* Bridge API */
  const corePid = ref(-1)
  const running = ref(false)
  const starting = ref(false)
  const stopping = ref(false)
  const restarting = ref(false)
  const needRestart = ref(false)
  const coreStateLoading = ref(true)
  let isCoreStartedByThisInstance = false
  let { promise: coreStoppedPromise, resolve: coreStoppedResolver } = Promise.withResolvers()
  // Bumped on every start, stop, and restart so a late sing-box exit cannot
  // revive routing after the user asked to stay stopped.
  let epoch = 0
  let userHold = false
  let releasing = false
  let startTicket = 0
  let stopTicket = 0
  let restartTicket = 0

  const stillCurrent = (ticket: number) => ticket === epoch && !userHold

  const armCoreStop = () => {
    coreStoppedPromise = new Promise((resolve) => {
      coreStoppedResolver = resolve
    })
  }

  const initCoreState = async () => {
    corePid.value = Number(await ReadFile(CorePidFilePath).catch(() => -1))
    const processName = corePid.value === -1 ? '' : await ProcessInfo(corePid.value).catch(() => '')
    running.value = processName.startsWith('sing-box')

    coreStateLoading.value = false

    if (running.value) {
      initWebsocket()
      await Promise.all([refreshConfig(), refreshProviderProxies()])
      await envStore.updateSystemProxyStatus()
    } else {
      // A crash can leave TUN routes and a loopback proxy with the core already dead.
      await CleanupNetworkAfterStop().catch(() => undefined)
      if (envStore.systemProxy) await envStore.clearSystemProxy().catch(() => undefined)
      if (appSettingsStore.app.autoStartKernel) {
        await startCore()
      }
    }
  }

  const runCoreProcess = async (isAlpha: boolean, ticket: number) => {
    let stopped = false
    const pid = await ExecBackground(
      CoreWorkingDirectory + '/' + getKernelFileName(isAlpha),
      getKernelRuntimeArgs(isAlpha),
      undefined,
      async (end) => {
        stopped = true
        const logs = await ReadFile(CoreLogFilePath, { Range: '-4096' }).catch((err) => String(err))
        logs.split('\n').forEach((line) => line && logsStore.recordKernelLog(line))
        end && logsStore.recordKernelLog(end)
        // A newer stop/start owns the process. Ignore this exit.
        if (ticket !== epoch) return
        await onCoreStopped()
      },
      {
        PidFile: CorePidFilePath,
        LogFile: CoreLogFilePath,
        Env: getKernelRuntimeEnv(isAlpha),
      },
    )
    const deadline = Date.now() + 20_000
    while (!stopped && Date.now() < deadline) {
      if (!stillCurrent(ticket)) break
      const ok = await probeApiAvailability().catch(() => false)
      if (ok) return pid
      await sleep(400)
    }

    if (pid > 0) await KillProcess(pid, 3).catch(() => undefined)
    if (!stopped && ticket === epoch) await onCoreStopped()
    if (!stillCurrent(ticket)) return 0
    throw t('kernel.startupFailed')
  }

  const onCoreStarted = async (pid: number, ticket: number) => {
    if (!stillCurrent(ticket)) return
    corePid.value = pid
    running.value = true
    needRestart.value = false
    isCoreStartedByThisInstance = true
    armCoreStop()

    initWebsocket()
    await Promise.all([refreshConfig(), refreshProviderProxies()])
    if (!stillCurrent(ticket)) {
      if (running.value && corePid.value === pid) await onCoreStopped()
      return
    }

    if (appSettingsStore.app.autoSetSystemProxy) {
      await envStore.setSystemProxy().catch((err) => message.error(err))
    }
    if (!stillCurrent(ticket)) {
      await envStore.clearSystemProxy().catch(() => undefined)
      if (running.value && corePid.value === pid) await onCoreStopped()
      return
    }
    if (appSettingsStore.app.autoSetSystemDNS) {
      await envStore.setSystemDNS(true).catch((err) => message.error(err))
    }
    if (!stillCurrent(ticket)) return
    await envStore.updateSystemProxyStatus()

    await pluginsStore.onCoreStartedTrigger()
  }

  const onCoreStopped = async () => {
    if (releasing) return
    if (!running.value && corePid.value < 0) {
      coreStoppedResolver(null)
      await CleanupNetworkAfterStop().catch(() => undefined)
      if (envStore.systemProxy) await envStore.clearSystemProxy().catch(() => undefined)
      return
    }
    releasing = true
    try {
      if (!isCoreStartedByThisInstance) {
        await RemoveFile(CorePidFilePath)
      }

      corePid.value = -1
      running.value = false
      needRestart.value = false

      destroyWebsocket()

      await envStore.updateSystemProxyStatus()
      if (envStore.systemProxy) {
        await envStore.clearSystemProxy().catch(() => undefined)
      }
      await CleanupNetworkAfterStop().catch(() => undefined)
      if (appSettingsStore.app.autoSetSystemDNS || envStore.systemDNSSet) {
        await envStore.setSystemDNS(false).catch((err) => message.error(err))
      }

      resetConfig()

      await Promise.race([
        pluginsStore.onCoreStoppedTrigger().catch(() => undefined),
        sleep(2000),
      ])

      coreStoppedResolver(null)
    } finally {
      releasing = false
    }
  }

  const ensureElevatedForTun = async () => {
    if (appSettingsStore.app.tunEnabled === false) return 'ok' as const
    if (envStore.env.os !== OS.Windows || envStore.env.isPrivileged) return 'ok' as const
    const restart = await confirm('settings.tunAdminTitle', 'settings.tunAdminBody', {
      type: 'text',
      okText: 'settings.tunAdminOk',
      cancelText: 'common.cancel',
    }).catch(() => false)
    if (restart) {
      await RunWithPowerShell(envStore.env.appPath, [], { admin: true, wait: false })
      await ExitApp()
      return 'leaving' as const
    }
    throw 'settings.tunAdminCancelled'
  }

  const launchCore = async (profileArg: App.Profile | undefined, ticket: number) => {
    if (!stillCurrent(ticket)) return
    if (running.value) throw 'The core is already running'

    logsStore.clearKernelLog()

    const { profile: profileID, branch } = appSettingsStore.app.kernel
    const profile = profileArg || profilesStore.getProfileById(profileID)
    if (!profile) throw 'Choose a profile first'

    if (!profileArg) {
      runtimeProfile = undefined
    }

    await generateConfigFile(profile, (config) =>
      pluginsStore.onBeforeCoreStartTrigger(config, profile),
    )
    if (!stillCurrent(ticket)) return
    const isAlpha = branch === Branch.Alpha
    const pid = await runCoreProcess(isAlpha, ticket)
    if (!pid || !stillCurrent(ticket)) return
    await onCoreStarted(pid, ticket)
  }

  const haltCore = async (ticket: number) => {
    await Promise.race([
      pluginsStore.onBeforeCoreStopTrigger().catch(() => undefined),
      sleep(2000),
    ])
    if (ticket !== epoch) return
    // Drop the system proxy before the process exits so traffic stops immediately.
    if (envStore.systemProxy) {
      await envStore.clearSystemProxy().catch(() => undefined)
    }
    const pid = corePid.value
    if (pid > 0) {
      await KillProcess(pid, 5).catch(() => undefined)
    }
    await CleanupNetworkAfterStop().catch(() => undefined)
    if (ticket !== epoch) return
    if (running.value || corePid.value > 0) {
      await Promise.race([coreStoppedPromise, sleep(1500)])
    }
    if (ticket !== epoch) return
    if (running.value || corePid.value > 0) {
      await onCoreStopped()
    }
  }

  const startCore = async (profile?: App.Profile) => {
    const ticket = ++epoch
    const mine = ++startTicket
    if ((await ensureElevatedForTun()) === 'leaving') return
    // A stop during the admin prompt bumps epoch and must keep the hold.
    if (ticket !== epoch) return
    userHold = false
    starting.value = true
    try {
      await launchCore(profile, ticket)
    } finally {
      if (mine === startTicket) starting.value = false
    }
  }

  const restoreNetworkAfterStop = async () => {
    await CleanupNetworkAfterStop().catch(() => undefined)
    if (envStore.systemProxy) await envStore.clearSystemProxy().catch(() => undefined)
  }

  const stopCore = async () => {
    const ticket = ++epoch
    const mine = ++stopTicket
    userHold = true
    restarting.value = false
    stopping.value = true
    try {
      if (!running.value && corePid.value <= 0) {
        await CleanupNetworkAfterStop().catch(() => undefined)
        if (envStore.systemProxy) await envStore.clearSystemProxy().catch(() => undefined)
        return
      }
      await haltCore(ticket)
    } finally {
      if (mine === stopTicket) stopping.value = false
    }
  }

  const restartCore = async (
    cleanupTask?: () => Promise<any>,
    keepRuntimeProfile = false,
    source: 'user' | 'auto' = 'user',
  ) => {
    if (source === 'auto' && (userHold || stopping.value)) return
    const ticket = ++epoch
    const mine = ++restartTicket
    restarting.value = true
    try {
      if ((await ensureElevatedForTun()) === 'leaving') return
      if (ticket !== epoch) return
      if (source === 'auto' && userHold) return
      if (source === 'user') userHold = false
      if (running.value || corePid.value > 0) await haltCore(ticket)
      if (!stillCurrent(ticket)) return
      await cleanupTask?.()
      if (!stillCurrent(ticket)) return
      await launchCore(keepRuntimeProfile ? runtimeProfile : undefined, ticket)
    } finally {
      if (mine === restartTicket) {
        needRestart.value = false
        restarting.value = false
      }
    }
  }

  const getProxyProfileOptions = (proxyType: ProxyType) => {
    const inboundTypeMap = {
      mixed: Inbound.Mixed,
      http: Inbound.Http,
      socks: Inbound.Socks,
    } satisfies Record<ProxyType, Inbound>

    const inbound = runtimeProfile?.inbounds.find(
      (item) => item.enable && item.type === inboundTypeMap[proxyType],
    )

    const inboundOptions =
      proxyType === Inbound.Mixed
        ? inbound?.mixed
        : proxyType === Inbound.Http
          ? inbound?.http
          : inbound?.socks

    const listen = inboundOptions?.listen.listen || ''
    const auth = inboundOptions?.users[0]?.trim()
    const host = normalizeProxyHost((listen || '').trim())

    if (!auth) return { host, username: '', password: '' }

    const [username, ...passwordParts] = auth.split(':')

    return {
      host,
      username: username || '',
      password: passwordParts.join(':'),
    }
  }

  const getProxyEndpoint = (): ProxyEndpoint | undefined => {
    const { port, 'socks-port': socksPort, 'mixed-port': mixedPort } = config.value
    let targetPort = 0
    let proxyType: ProxyType | undefined

    if (mixedPort) {
      targetPort = mixedPort
      proxyType = 'mixed'
    } else if (port) {
      targetPort = port
      proxyType = 'http'
    } else if (socksPort) {
      targetPort = socksPort
      proxyType = 'socks'
    } else {
      return undefined
    }

    const { host, username, password } = getProxyProfileOptions(proxyType)
    const schema = proxyType === 'socks' ? 'socks5' : 'http'

    return {
      schema,
      host,
      port: targetPort,
      username,
      password,
      proxyType,
    }
  }

  eventBus.on('profileChange', ({ id }) => {
    if (running.value && id === appSettingsStore.app.kernel.profile) {
      needRestart.value = true
    }
  })

  eventBus.on('subscriptionChange', ({ id }) => {
    if (running.value && profilesStore.currentProfile) {
      const inUse = profilesStore.currentProfile.outbounds.some(({ outbounds }) =>
        outbounds.some((outbound) => outbound.type === 'Subscription' && outbound.id === id),
      )
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  eventBus.on('subscriptionsChange', () => {
    if (running.value && profilesStore.currentProfile) {
      const enabledSubs = subscribesStore.subscribes.flatMap((v) => (v.disabled ? [] : v.id))
      const inUse = profilesStore.currentProfile.outbounds.some(({ outbounds }) =>
        outbounds.some(
          (outbound) => outbound.type === 'Subscription' && enabledSubs.includes(outbound.id),
        ),
      )
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  const collectRulesetIDs = () => {
    if (!profilesStore.currentProfile) return []
    const l1 = profilesStore.currentProfile.route.rule_set.flatMap((ruleset) =>
      ruleset.type === RulesetType.Local ? ruleset.path : [],
    )
    return l1
  }

  eventBus.on('rulesetChange', ({ id }) => {
    if (running.value && profilesStore.currentProfile) {
      const inUse = profilesStore.currentProfile.route.rule_set.some(
        (ruleset) => ruleset.type === RulesetType.Local && ruleset.path === id,
      )
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  eventBus.on('rulesetsChange', () => {
    if (running.value && profilesStore.currentProfile) {
      const enabledRulesets = rulesetsStore.rulesets.flatMap((v) => (v.disabled ? [] : v.id))
      const inUse = collectRulesetIDs().some((v) => enabledRulesets.includes(v))
      if (inUse) {
        needRestart.value = true
      }
    }
  })

  watch(needRestart, (v) => {
    if (v && appSettingsStore.app.autoRestartKernel && !userHold && !stopping.value) {
      restartCore(undefined, false, 'auto').catch((error) => console.error(error))
    }
  })

  const watchSources = computed(() => {
    const source = [config.value.mode, config.value.tun.enable]
    if (!appSettingsStore.app.addGroupToMenu) return source.join('')

    const { unAvailable, sortByDelay } = appSettingsStore.app.kernel

    const proxySignature = Object.values(proxies.value)
      .map((group) => group.name + group.now)
      .sort()
      .join()

    return source.concat([proxySignature, unAvailable, sortByDelay]).join('')
  })

  watch([watchSources, running], updateTrayAndMenus)

  return {
    startCore,
    stopCore,
    restartCore,
    restoreNetworkAfterStop,
    initCoreState,
    pid: corePid,
    running,
    starting,
    stopping,
    restarting,
    needRestart,
    coreStateLoading,
    config,
    proxies,
    refreshConfig,
    updateConfig,
    refreshProviderProxies,
    getProxyEndpoint,

    onLogs,
    onMemory,
    onTraffic,
    onConnections,
  }
})
