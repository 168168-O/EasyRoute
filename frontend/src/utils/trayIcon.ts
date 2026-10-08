export const trayIconFile = (input: {
  os: string
  theme: string
  running: boolean
  tun: boolean
  proxy: boolean
  alert: boolean
}) => {
  const linux = input.os === 'linux'
  const ext = linux ? '.png' : '.ico'
  const folder = linux ? 'imgs' : 'icons'
  const mark = input.alert ? '_alert' : ''
  let name = 'tray_normal'
  if (input.running && input.tun) name = 'tray_tun'
  else if (input.running && input.proxy) name = 'tray_proxy'
  return `data/.cache/${folder}/${name}${mark}_${input.theme}${ext}`
}
