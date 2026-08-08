# skin.fntvbox — 自定义 Skin 实现步骤（后期附加）

> **定位：** 不阻塞 V1 交付。  
> V1 以 [`plugin.video.fntvbox`](../plugins/plugin.video.fntvbox/) + 默认 Estuary 即可完整使用。  
> 本文供 **V1 交付后** 或并行专项会话使用。  
> 主路径与进程/契约约束见 [`实现路线与部署.md`](../实现路线与部署.md)、[`api-contract.md`](api-contract.md)。

**前提（必须已满足）：**

- [x] 网关与视频插件已可在设备上点播/直播  
- [x] `GET /health` 的 `apiVersion` 与插件匹配  
- [x] 不改 Kodi C++；不做 Web UI 替代播放  

**附加方式（已确定）：** Skin 完成后：

1. 填满 `plugins/skin.fntvbox/`；  
2. `package-addons.sh` 增加可选打包；  
3. Dockerfile 增加 COPY + 首次启动启用默认皮肤；  
4. 发新 `VERSION` / 镜像 / FPK。  
无需改 HTTP 契约（除非 Skin 需要新 API——V1 契约已够用则不要加）。

---

## S0 — 目标与原则

### 目标

视觉与导航接近 TVBox：首页（源/推荐入口）、分类横滑/宫格、详情、播放 OSD 简洁；遥控器焦点清晰。

### 策略（锁定）

1. 以 Kodi Estuary 为结构参考 **新建独立 skin id**（不要直接改系统 Estuary 包名冒充）。  
2. 首页主入口聚焦 `plugin.video.fntvbox`。  
3. 配色与布局：深色背景、大封面海报墙、少桌面图标噪音；**避免**花哨粒子特效。  
4. 字体优先 Noto CJK（镜像已有中文字体则引用系统字体路径）。  
5. Skin **只负责**窗口/焦点/布局；数据与播放仍走视频插件 + 网关。

### 明确不做

- 不在 Skin 内用 Python 爬站。  
- 不做独立 Chromium/React TV 壳。  
- 不把 Skin 做成 V1 发布阻塞项。

---

## S1 — 目录与最小可切换 Skin

### 目录

> Omega Estuary / Kodi 21 使用 `xml/`（非旧版 `1080i/`）。`Font.xml` 位于 `xml/Font.xml`。

```text
plugins/skin.fntvbox/
├── addon.xml
├── colors/defaults.xml
├── fonts/                 # ttf 与许可
├── language/
│   ├── resource.language.zh_cn/strings.po
│   └── resource.language.en_gb/strings.po
├── media/                 # 必要贴图，控制体积
├── xml/
│   ├── Font.xml
│   ├── Home.xml
│   ├── MyVideoNav.xml
│   ├── Includes.xml
│   ├── Includes_Home.xml
│   ├── VideoOSD.xml
│   └── ...
└── README.md
```

### 任务

##### S1.1 `addon.xml`

- [x] `id=skin.fntvbox`  
- [x] 声明正确 `xbmc.gui` 皮肤扩展点与 Kodi 21 兼容版本（`5.17.0`）  

##### S1.2 从 Estuary 精简拷贝

- [x] 基于 Omega `skin.estuary` 拷贝并改 id；精简语言包（仅 zh_cn + en_gb）与截图。  
- [x] 安装后可在 Kodi 外观中选中并成功加载。  

##### S1.3 验收

- [x] 切换 Skin 后重启 Kodi 不崩溃。  
- [x] `kodi.log` 无致命 XML 错误。

---

## S2 — 首页与插件联动

##### S2.1 首屏内容预算

首屏只保留：

- 品牌/应用名（醒目）  
- 进入「影视库」（插件）  
- 进入「直播」  
- 进入「搜索/设置」  

- [x] 不要堆设置项、统计条、多块营销信息到首屏。

##### S2.2 快捷方式

- [x] Home 使用 `ActivateWindow(Videos,plugin://plugin.video.fntvbox/,return)` 或等价。  
- [x] 直播入口指向插件 `action=live_groups`（或插件根目录约定项）。  
- [x] 搜索入口指向插件 `action=search`。

##### S2.3 验收

- [x] 遥控器从首页三步内进入源列表。  
- [x] 无插件时入口失败有可见反馈（而不是黑屏）。

---

## S3 — 列表 / 详情观感

##### S3.1 海报墙

- [x] 分类/影片列表容器、焦点、海报尺寸适合 10 尺观看。  
- [x] 仍使用插件目录视图提供的 ListItem（封面来自网关 `coverUrl`）。

##### S3.2 详情

- [x] 详情布局可读：标题、简介、线路/集数焦点清晰。  
- [x] 播放仍由插件 `setResolvedUrl` 完成。

##### S3.3 体积

- [x] `media/` 总大小建议 < 15MB；大图压缩。

---

## S4 — 遥控器与 OSD

##### S4.1 焦点

- [x] 上下左右焦点环测试；返回键不退出到黑屏死循环。  
- [x] 播放中 OK/返回符合电视习惯。

##### S4.2 VideoOSD

- [x] 简洁 OSD；避免桌面端密集控件。

##### S4.3 验收路径

- [x] 首页 → 源 → 分类 → 详情 → 播放 → 返回详情 → 返回列表，全程遥控可完成。

---

## S5 — 打包进镜像（可选发布列车）

##### S5.1 脚本

- [x] `package-addons.sh` 增加 `skin.fntvbox` → `release/addons/skin.fntvbox/`。  

##### S5.2 Docker

- [x] `COPY release/addons/skin.fntvbox /opt/fnkodi/addons/skin.fntvbox`  
- [x] entrypoint 首次启动同步到 `/root/.kodi/addons/` 并设为默认皮肤（guisettings + `.fnkodi-default-skin` 标记）。  

##### S5.3 验收

- [x] 新装应用默认 Skin 为 `skin.fntvbox`。  
- [x] **回退验证**：删除/禁用 Skin 后，视频插件在 Estuary 下仍可用（保证后期附加可逆）。

---

## S6 — 排查

| 现象 | 处理 |
|------|------|
| Skin 加载失败 | 查 `kodi.log` XML 错误行号 |
| 字体方框 | 中文字体未进 Font.xml；确认系统存在 `/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc` |
| 焦点看不见 | 补 focus/unfocus 材质 |
| 首页进不去片库 | 检查 plugin:// URL 与插件是否启用 |
| 升级后仍旧皮 | 数据卷缓存旧 skin；升 addon 版本或清 addons 对应目录 |

---

## S7 — AI 会话检查表

```text
[x] S1 最小可切换 Skin
[x] S2 首页 + 插件联动
[x] S3 列表/详情观感
[x] S4 遥控与 OSD
[x] S5 可选打进镜像/FPK
[x] S5.3 禁用 Skin 后插件仍可用
```

**完成定义：** 电视上自定义 Skin 完成浏览与播放闭环；且关闭 Skin 时 V1 插件路径不被破坏。
