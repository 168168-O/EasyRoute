# 达货爱vpn姑娘

Based on GUI.for.SingBox (GPL-3.0).

Windows 图形界面：导入订阅后连接，再按软件选择走代理或走本地。本仓库保留上游版权与 GPL-3.0，产品名称是「达货爱vpn姑娘」。

## Preview

Take a look at the live version here: 👉 <a href="https://gui-for-cores.github.io/guide/gfs/" target="_blank">Live Demo</a>

<div align="center">
  <img src="docs/imgs/light.png">
</div>

## Document

[Community](https://gui-for-cores.github.io/guide/gfs/community)

## Build

1、Build Environment

- Node.js [link](https://nodejs.org/en)

- pnpm ：`npm i -g pnpm`

- Go [link](https://go.dev/)

- Wails [link](https://wails.io/) ：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`

2、Pull and Build

```bash
git clone https://github.com/168168-O/EasyRoute.git

cd EasyRoute/frontend

pnpm install --frozen-lockfile && pnpm build

cd ..

wails build
```