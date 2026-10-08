import { ExecBackground, FileExists, HttpGet, KillProcess, RemoveFile, WriteFile } from '@/bridge'
import { CoreWorkingDirectory } from '@/constant/kernel'
import { sleep, getKernelFileName } from '@/utils'

import type { SingboxOutbound } from './subscriptionConvert.ts'

const TEST_URL = 'https://www.gstatic.com/generate_204'
let nextPort = 18620

const takePort = () => {
  nextPort += 1
  if (nextPort > 19620) nextPort = 18621
  return nextPort
}

const probeConfig = (node: SingboxOutbound, port: number) => ({
  log: { level: 'error', timestamp: false },
  inbounds: [
    {
      type: 'mixed',
      tag: 'probe-in',
      listen: '127.0.0.1',
      listen_port: port,
    },
  ],
  outbounds: [node],
  route: { final: node.tag },
})

/** One short sing-box, no TUN and no system proxy, then it exits. */
export const probeFreeNode = async (node: SingboxOutbound): Promise<{ ok: boolean; delayMs: number }> => {
  const binary = `${CoreWorkingDirectory}/${getKernelFileName(false)}`
  if (!(await FileExists(binary).catch(() => false))) return { ok: false, delayMs: 0 }

  const port = takePort()
  const configPath = `${CoreWorkingDirectory}/free-probe-${port}.json`
  const logPath = `${CoreWorkingDirectory}/free-probe-${port}.log`
  let pid = 0
  try {
    await WriteFile(configPath, JSON.stringify(probeConfig(node, port)))
    pid = await ExecBackground(binary, ['run', '-c', configPath, '-D', CoreWorkingDirectory], undefined, undefined, {
      LogFile: logPath,
    })
    const started = Date.now()
    for (let attempt = 0; attempt < 6; attempt += 1) {
      try {
        const response = await HttpGet(TEST_URL, {}, { Proxy: `http://127.0.0.1:${port}`, Timeout: 4 })
        const delayMs = Date.now() - started
        const ok = response.status > 0 && response.status < 400
        return { ok, delayMs: ok ? delayMs : 0 }
      } catch {
        await sleep(250)
      }
    }
    return { ok: false, delayMs: 0 }
  } catch {
    return { ok: false, delayMs: 0 }
  } finally {
    if (pid > 0) await KillProcess(pid, 3).catch(() => undefined)
    await RemoveFile(configPath).catch(() => undefined)
    await RemoveFile(logPath).catch(() => undefined)
  }
}
