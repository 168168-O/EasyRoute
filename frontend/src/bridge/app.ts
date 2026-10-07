import * as Bridge from '@wails/go/bridge/App'
import {
  IsNotificationAvailable,
  RequestNotificationAuthorization,
  SendNotification,
} from '@wails/runtime/runtime'

import { sampleID } from '@/utils'

export const RestartApp = Bridge.RestartApp

export const ExitApp = Bridge.ExitApp

export const ShowMainWindow = Bridge.ShowMainWindow

export const UpdateTray = Bridge.UpdateTray

export const UpdateTrayMenus = Bridge.UpdateTrayMenus

export const UpdateTrayAndMenus = Bridge.UpdateTrayAndMenus

export const GetEnv = <T extends string | undefined = undefined>(
  key?: T,
): Promise<T extends string ? string : App.AppEnv> => {
  return Bridge.GetEnv(key || '')
}

export const IsStartup = Bridge.IsStartup

export const GetSystemProxy = async () => {
  const { flag, data } = await Bridge.GetSystemProxy()
  if (!flag) {
    throw data
  }
  return data
}

export const SetSystemProxy = async (
  enable: boolean,
  server: string,
  proxyType: 'mixed' | 'http' | 'socks' = 'mixed',
  bypass = '',
  services: string[] = [],
) => {
  const { flag, data } = await Bridge.SetSystemProxy(enable, server, proxyType, bypass, services)
  if (!flag) {
    throw data
  }
  return data
}

export const SetSystemDNS = async (servers: string, services: string[] = []) => {
  const { flag, data } = await Bridge.SetSystemDNS(servers, services)
  if (!flag) {
    throw data
  }
  return data
}

export const GetSystemProxyBypass = async () => {
  const { flag, data } = await Bridge.GetSystemProxyBypass()
  if (!flag) {
    throw data
  }
  return data
}

export const GetInterfaces = async () => {
  const { flag, data } = await Bridge.GetInterfaces()
  if (!flag) {
    throw data
  }
  return data.split('|')
}

export const ListProcesses = async () => {
  const { flag, data } = await Bridge.ListProcesses()
  if (!flag) throw data
  return JSON.parse(data) as { name: string; exe: string }[]
}

export const ListDouyinExes = async () => {
  const { flag, data } = await Bridge.ListDouyinExes()
  if (!flag) throw data
  return data as string
}

export const PickFile = async (title: string, pattern: string) => {
  const { flag, data } = await Bridge.PickFile(title, pattern)
  if (!flag) throw data
  return data as string
}

export const BackgroundVideoURL = async () => {
  const { flag, data } = await Bridge.BackgroundVideoURL()
  if (!flag) throw data
  return data as string
}

export const Translate = async (text: string, target: string, coreProxy = '') => {
  const { flag, data } = await Bridge.Translate(text, target, coreProxy)
  if (!flag) throw data
  return JSON.parse(data) as {
    text: string
    sourceLang: string
    targetLang: string
    provider: string
  }
}

export const Notify = async (title: string, body: string) => {
  if (!(await IsNotificationAvailable())) {
    throw new Error('Notifications not available on this platform')
  }
  await RequestNotificationAuthorization()
  await SendNotification({ id: sampleID(), title, body })
}
