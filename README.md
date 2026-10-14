# 达货爱vpn姑娘

基于 GUI.for.SingBox（GPL-3.0）。本仓库保留上游版权。产品名称是「达货爱vpn姑娘」，界面固定为磨砂玻璃和牛头图标。

Windows 图形界面：导入订阅后连接，再按软件选择走代理或走本地。

## 构建

1、环境

- Node.js [link](https://nodejs.org/en)
- pnpm：`npm i -g pnpm`
- Go [link](https://go.dev/)
- Wails [link](https://wails.io/)：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`

2、拉代码并构建

```bash
git clone https://github.com/168168-O/EasyRoute.git

cd EasyRoute/frontend

pnpm install --frozen-lockfile && pnpm build

cd ..

wails build
```
