# skin.fntvbox

FnKodi TVBox 自定义 Kodi Skin（基于 Kodi 21 / Omega Estuary，`xbmc.gui` 5.17.0）。

## 结构说明

Omega Estuary 使用 `xml/` 目录（非旧版 `1080i/`）。窗口 XML、`Font.xml` 均在 `xml/` 下；贴图在 `media/`。

## 功能

- 首页聚焦：影视库 / 直播 / 搜索 / 设置 → `plugin.video.fntvbox`
- 深色配色、大封面海报墙（`View_51_Poster`）、简洁 VideoOSD
- 字体优先引用系统 Noto CJK：`/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc`（无该文件时需安装 Noto CJK 或改回皮肤自带 `NotoSans-Regular.ttf`）

## 安装与切换

1. 打包：`bash scripts/package-addons.sh` → `release/addons/skin.fntvbox/`
2. 镜像预装后，entrypoint 首次启动设为默认皮肤
3. 手动切换：设置 → 界面 → 皮肤 → FnKodi TVBox

## 回退

禁用或删除本皮肤后，基座镜像内系统 Estuary 仍可用；`plugin.video.fntvbox` 路径不受影响。

## 许可

基于 Team Kodi Estuary（CC BY-SA 4.0 / GPL-2.0），见 `LICENSE.txt`。
