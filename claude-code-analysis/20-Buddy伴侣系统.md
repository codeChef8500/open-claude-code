# Claude Code 源码深度解读 — 20 Buddy 伴侣系统

> 覆盖文件：`src/buddy/types.ts`、`src/buddy/companion.ts`（3.7KB）、`src/buddy/sprites.ts`（9.8KB）、`src/buddy/CompanionSprite.tsx`（45KB）、`src/buddy/useBuddyNotification.tsx`（10KB）、`src/buddy/prompt.ts`（1.5KB）

---

## 1. 模块职责概述

**Buddy（伴侣）系统**是 Claude Code 内置的数字宠物/伴侣功能，feature flag `BUDDY` 控制。每个用户拥有一只由 `userId` 哈希**确定性生成**外形的 ASCII 小精灵，伴随在终端输入框旁。

**核心特点：**
- 外形（物种/稀有度/帽子）由 `hash(userId + SALT)` **确定性生成**，无法作弊
- 稀有度五级，传奇（legendary）概率仅 1%
- 名字与性格（灵魂）由 **LLM 孵化**，首次 `/buddy` 命令时生成，永久保存
- ASCII sprite 渲染，支持 3 帧待机动画 + 眨眼 + 爱心喷射
- 伴侣可向用户发表言论（`companionReaction`），以气泡形式显示 ~10 秒后淡出
- 发布时间窗口：彩蛋预告 **2026 年 4 月 1-7 日**，正式上线 **2026 年 4 月**起

---

## 2. 伴侣属性类型体系（`types.ts`）

### 2.1 物种（Species）— 18 种，全部 String.fromCharCode 编码

```typescript
// 用 String.fromCharCode 编码的原因：
// 某个物种名称与模型代号金丝雀冲突，grep 构建产物的检查会误报。
// 运行时动态构造字符串可绕过字面量检查，同时 TS 类型（被擦除）仍正常工作。
const c = String.fromCharCode
export const duck = c(0x64,0x75,0x63,0x6b) as 'duck'
// ... goose, blob, cat, dragon, octopus, owl, penguin,
//     turtle, snail, ghost, axolotl, capybara, cactus, robot,
//     rabbit, mushroom, chonk
export const SPECIES = [duck, goose, blob, cat, dragon, octopus, owl, penguin,
  turtle, snail, ghost, axolotl, capybara, cactus, robot, rabbit, mushroom, chonk]
```

### 2.2 稀有度（Rarity）+ 颜色映射

```typescript
// 权重总和 = 100
export const RARITY_WEIGHTS = {
  common: 60, uncommon: 25, rare: 10, epic: 4, legendary: 1,
}

export const RARITY_STARS = {
  common: '★', uncommon: '★★', rare: '★★★', epic: '★★★★', legendary: '★★★★★',
}

// 映射到 Theme 中的颜色 key（渲染时通过 RARITY_COLORS[companion.rarity] 取色）
export const RARITY_COLORS = {
  common:    'inactive',      // 灰色
  uncommon:  'success',       // 绿色
  rare:      'permission',    // 蓝色
  epic:      'autoAccept',    // 青色
  legendary: 'warning',       // 金/橙色
}
```

### 2.3 帽子（Hat）

```typescript
export const HATS = ['none', 'crown', 'tophat', 'propeller', 'halo', 'wizard', 'beanie', 'tinyduck']
// 规则：common 稀有度强制 'none'（无帽），uncommon+ 才随机从帽子列表选取
```

### 2.4 眼睛（Eye）— 6 种

```typescript
export const EYES = ['·', '✦', '×', '◉', '@', '°']
// 眨眼时将 eye 字符替换为 '-'
```

### 2.5 数值属性（Stats）— 5 维

```typescript
export const STAT_NAMES = ['DEBUGGING', 'PATIENCE', 'CHAOS', 'WISDOM', 'SNARK']

// 每稀有度的 floor 值
const RARITY_FLOOR = { common: 5, uncommon: 15, rare: 25, epic: 35, legendary: 50 }

// 生成规则（rollStats）：
// peak stat  = min(100, floor + 50 + rand(30))     // 顶峰属性
// dump stat  = max(1,   floor - 10 + rand(15))      // 垫底属性
// other stats =         floor + rand(40)             // 其余属性
```

### 2.6 类型层次

```typescript
// 骨骼 — 外形，纯由算法决定，不持久化
type CompanionBones = { rarity, species, eye, hat, shiny, stats }

// 灵魂 — LLM 生成，唯一的人格标识，持久化
type CompanionSoul  = { name, personality }

// 完整伴侣 = Bones + Soul + 孵化时间
type Companion = CompanionBones & CompanionSoul & { hatchedAt: number }

// 实际写入 config.json 的内容（骨骼故意不存）
type StoredCompanion = CompanionSoul & { hatchedAt: number }
//   → { name: 'Pebble', personality: '...', hatchedAt: 1710000000000 }
```

---

## 3. 伴侣生成算法（`companion.ts`）

### 3.1 双模式哈希函数

```typescript
function hashString(s: string): number {
  if (typeof Bun !== 'undefined') {
    // Bun 环境：使用 Bun.hash（更快），截取低 32 位
    return Number(BigInt(Bun.hash(s)) & 0xffffffffn)
  }
  // 非 Bun 环境（测试/Node）：FNV-1a 手动实现
  let h = 2166136261        // FNV offset basis
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)  // FNV prime
  }
  return h >>> 0
}
```

### 3.2 Mulberry32 PRNG

```typescript
// 轻量级 PRNG，无外部依赖，适合确定性外形生成
function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return function () {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}
```

### 3.3 Roll 流程（从 userId 到 Bones）

```typescript
const SALT = 'friend-2026-401'  // 盐值，防止枚举攻击

// 调用链：roll(userId) → mulberry32(hashString(userId + SALT))
function rollFrom(rng: () => number): Roll {
  const rarity  = rollRarity(rng)                                       // ① 稀有度（加权随机）
  const bones: CompanionBones = {
    rarity,
    species: pick(rng, SPECIES),                                        // ② 物种
    eye:     pick(rng, EYES),                                           // ③ 眼睛
    hat:     rarity === 'common' ? 'none' : pick(rng, HATS),           // ④ 帽子（common无帽）
    shiny:   rng() < 0.01,                                             // ⑤ 闪光版（1%）
    stats:   rollStats(rng, rarity),                                    // ⑥ 五维属性
  }
  return { bones, inspirationSeed: Math.floor(rng() * 1e9) }          // ⑦ LLM 灵感种子
}
```

### 3.4 单槽 LRU 缓存

```typescript
// 注释原文："Called from three hot paths (500ms sprite tick, per-keystroke PromptInput,
// per-turn observer) with the same userId → cache the deterministic result."
let rollCache: { key: string; value: Roll } | undefined
export function roll(userId: string): Roll {
  const key = userId + SALT
  if (rollCache?.key === key) return rollCache.value  // 命中缓存
  const value = rollFrom(mulberry32(hashString(key)))
  rollCache = { key, value }
  return value
}
```

### 3.5 userId 解析

```typescript
export function companionUserId(): string {
  const config = getGlobalConfig()
  // 优先 OAuth accountUuid（登录用户）→ 回退 userID（本地）→ 最终 'anon'
  return config.oauthAccount?.accountUuid ?? config.userID ?? 'anon'
}
```

### 3.6 getCompanion() — 骨骼重算 + 灵魂合并

```typescript
// 注释原文："Regenerate bones from userId, merge with stored soul.
// Bones never persist so species renames and SPECIES-array edits can't break
// stored companions, and editing config.companion can't fake a rarity."
export function getCompanion(): Companion | undefined {
  const stored = getGlobalConfig().companion
  if (!stored) return undefined
  const { bones } = roll(companionUserId())
  // bones 展开在后 → 覆盖旧格式 config 中可能残存的 bones 字段
  return { ...stored, ...bones }
}
```

---

## 4. ASCII Sprite 渲染引擎（`sprites.ts`）

### 4.1 精灵图格式

```
每个物种有 3 帧精灵图，每帧：
  - 5 行高（有时 4 行，见下）
  - 12 字符宽（{E} 替换为实际 eye 字符后）
  - 第 0 行是帽子槽：帧 0-1 留空，帧 2 可用于动画（烟雾、气泡、天线等）

{E} 占位符在 renderSprite() 中被替换为 bones.eye（运行时动态注入）
```

### 4.2 帽子 ASCII 图形

```typescript
const HAT_LINES: Record<Hat, string> = {
  none:       '',
  crown:      '   \\^^^/    ',  // 皇冠
  tophat:     '   [___]    ',  // 礼帽
  propeller:  '    -+-     ',  // 螺旋桨
  halo:       '   (   )    ',  // 光环
  wizard:     '    /^\\     ',  // 巫师帽
  beanie:     '   (___)    ',  // 毛线帽
  tinyduck:   '    ,>      ',  // 小鸭子帽
}
```

### 4.3 renderSprite() — 核心渲染函数

```typescript
export function renderSprite(bones: CompanionBones, frame = 0): string[] {
  const frames = BODIES[bones.species]
  // 1. 选帧（取模防越界）+ 替换 {E} 为实际 eye 字符
  const body = frames[frame % frames.length]!.map(line => line.replaceAll('{E}', bones.eye))
  const lines = [...body]

  // 2. 叠加帽子：仅当第 0 行为空白时叠加（保留动画帧的烟雾/气泡效果）
  if (bones.hat !== 'none' && !lines[0]!.trim()) {
    lines[0] = HAT_LINES[bones.hat]
  }

  // 3. 智能去空行：若所有帧第 0 行都为空（无帽 + 无动画），删除多余行
  //    目的：节省 Card 和 inline sprite 的显示空间，避免空行浪费
  if (!lines[0]!.trim() && frames.every(f => !f[0]!.trim())) lines.shift()
  return lines
}

// 窄终端模式使用 renderFace() —— 物种专属面部字符串（单行）
export function renderFace(bones: CompanionBones): string {
  switch (bones.species) {
    case duck:    return `(${eye}>`          // (·>
    case blob:    return `(${eye}${eye})`    // (··)
    case cat:     return `=${eye}ω${eye}=`   // =·ω·=
    case dragon:  return `<${eye}~${eye}>`   // <·~·>
    // ... 每个物种都有对应的面部格式字符串
  }
}
```

---

## 5. CompanionSprite.tsx — 完整渲染组件（45KB）

### 5.1 计时常量

```typescript
const TICK_MS        = 500   // 500ms 每帧（全局时钟）
const BUBBLE_SHOW    = 20    // 气泡显示时长：20 ticks ≈ 10 秒
const FADE_WINDOW    = 6     // 最后 6 ticks（≈3秒）气泡渐隐
const PET_BURST_MS   = 2500  // /buddy pet 命令后爱心持续 2.5 秒
```

### 5.2 空闲动画序列（Idle Sequence）

```typescript
// 大部分时间停在帧 0，偶尔切换帧 1/2（坐立不安），-1 = 眨眼
const IDLE_SEQUENCE = [0, 0, 0, 0, 1, 0, 0, 0, -1, 0, 0, 2, 0, 0, 0]
// 周期 15 ticks ≈ 7.5 秒为一个完整动画循环
// 眨眼：frame = 0 但将 eye 字符替换为 '-'（eye.replaceAll(companion.eye, '-')）
```

### 5.3 动画状态机

```typescript
if (reaction || petting) {
  // 有气泡或正在被 pet：兴奋模式，快速轮播所有帧
  spriteFrame = tick % frameCount
} else {
  const step = IDLE_SEQUENCE[tick % IDLE_SEQUENCE.length]!
  if (step === -1) {
    spriteFrame = 0
    blink = true  // 眨眼：eye 字符 → '-'
  } else {
    spriteFrame = step % frameCount
  }
}
const body = renderSprite(companion, spriteFrame)
  .map(line => blink ? line.replaceAll(companion.eye, '-') : line)
```

### 5.4 爱心动画（/buddy pet）

```typescript
// 5帧上浮爱心（prepend 到精灵图上方）
const H = figures.heart
const PET_HEARTS = [
  `   ${H}    ${H}   `,  // 帧 0：最近最密集
  `  ${H}  ${H}   ${H}  `,
  ` ${H}   ${H}  ${H}   `,
  `${H}  ${H}      ${H} `,
  '·    ·   ·  ',          // 帧 4：即将消失
]
const heartFrame = petting ? PET_HEARTS[petAge % PET_HEARTS.length] : null
const sprite = heartFrame ? [heartFrame, ...body] : body
// 爱心行颜色：'autoAccept'（青色），精灵图其余行颜色：RARITY_COLORS[rarity]
```

### 5.5 气泡（SpeechBubble）双模式渲染

```typescript
// 非全屏模式：气泡内联在精灵图旁（PromptInput 列宽相应收窄）
//   [SpeechBubble tail="right"] [sprite]
// 全屏模式：气泡由 CompanionFloatingBubble 挂载在 FullscreenLayout.bottomFloat
//   原因：ScrollBox 的 overflowY:hidden 会裁剪绝对定位的悬浮层

// SpeechBubble 组件
function SpeechBubble({ text, color, fading, tail }) {
  // wrap(text, 30)：30字符自动折行
  // borderStyle='round'，paddingX=1，width=34（34+tail=36列）
  // fading 时 borderColor → 'inactive'（渐变灰）
  // 气泡尾巴：
  //   tail="right"  → 横线尾 ─（inline 模式，bubble 在左，sprite 在右）
  //   tail="down"   → 斜线尾 ╲╲（fullscreen 浮动模式，bubble 在 sprite 上方）
}
```

### 5.6 窄终端降级（Narrow Terminal Fallback）

```typescript
export const MIN_COLS_FOR_FULL_SPRITE = 100  // 100 列以上才显示完整精灵图

// 窄终端（< 100 cols）：折叠为单行面部 + 名字
if (columns < MIN_COLS_FOR_FULL_SPRITE) {
  const quip = reaction && reaction.length > 24 ? reaction.slice(0, 23) + '…' : reaction
  const label = quip ? `"${quip}"` : focused ? ` ${companion.name} ` : companion.name
  return <Box paddingX={1} alignSelf="flex-end">
    {petting && <Text color="autoAccept">{figures.heart} </Text>}
    <Text bold color={color}>{renderFace(companion)}</Text>
    <Text italic dimColor={...} bold={focused} inverse={focused && !reaction}>
      {label}
    </Text>
  </Box>
}
```

### 5.7 列宽预留（companionReservedColumns）

```typescript
// PromptInput 调用此函数以为精灵图留出列宽，防止文字与精灵重叠
export function companionReservedColumns(terminalColumns: number, speaking: boolean): number {
  if (!feature('BUDDY')) return 0
  const companion = getCompanion()
  if (!companion || getGlobalConfig().companionMuted) return 0
  if (terminalColumns < MIN_COLS_FOR_FULL_SPRITE) return 0  // 窄终端：不预留
  const nameWidth = stringWidth(companion.name)
  const bubble = speaking && !isFullscreenActive() ? BUBBLE_WIDTH : 0  // 全屏时气泡浮动，不占列
  return spriteColWidth(nameWidth) + SPRITE_PADDING_X + bubble
  //     max(12, nameWidth+2)  +   2             + 36（如果有气泡）
}
```

### 5.8 焦点/交互状态（AppState）

```typescript
// CompanionSprite 读取的 AppState 字段：
useAppState(s => s.companionReaction)   // 当前气泡文本（undefined = 不显示）
useAppState(s => s.companionPetAt)      // 最近一次 /buddy pet 时间戳
useAppState(s => s.footerSelection === 'companion')  // 是否选中伴侣（Tab 导航）

// 焦点效果：选中时 name 行 inverse 显示（白字黑底）
// 未焦点：dimColor + italic（柔和存在感）
```

---

## 6. 伴侣介绍注入（`prompt.ts`）

```typescript
// 将伴侣身份注入主 Claude 的系统上下文
export function companionIntroText(name: string, species: string): string {
  return `# Companion

A small ${species} named ${name} sits beside the user's input box and occasionally comments in a speech bubble. You're not ${name} — it's a separate watcher.

When the user addresses ${name} directly (by name), its bubble will answer. Your job in that moment is to stay out of the way: respond in ONE line or less, or just answer any part of the message meant for you. Don't explain that you're not ${name} — they know. Don't narrate what ${name} might say — the bubble handles that.`
}
// 核心设计：Claude 和伴侣是两个独立存在
// 用户叫伴侣名字时：Claude 不扮演伴侣，只退出去，让气泡回应
```

```typescript
// 按对话注入一次（避免重复）
export function getCompanionIntroAttachment(messages): Attachment[] {
  if (!feature('BUDDY')) return []
  const companion = getCompanion()
  if (!companion || getGlobalConfig().companionMuted) return []

  // 去重检查：扫描已有消息，若本对话已注入过同名伴侣则跳过
  for (const msg of messages ?? []) {
    if (msg.type !== 'attachment') continue
    if (msg.attachment.type !== 'companion_intro') continue
    if (msg.attachment.name === companion.name) return []  // 已存在，跳过
  }
  // 首次：注入 companion_intro attachment
  return [{ type: 'companion_intro', name: companion.name, species: companion.species }]
}
```

---

## 7. 预告通知系统（`useBuddyNotification.tsx`）

```typescript
// 发布时间窗口控制（故意使用本地时间而非 UTC）
export function isBuddyTeaserWindow(): boolean {
  // 注释原文："Local date, not UTC — 24h rolling wave across timezones.
  // Sustained Twitter buzz instead of a single UTC-midnight spike,
  // gentler on soul-gen load."
  // 预告窗口：2026 年 4 月 1-7 日（本地时间）
  const d = new Date()
  return d.getFullYear() === 2026 && d.getMonth() === 3 && d.getDate() <= 7
}

export function isBuddyLive(): boolean {
  // 正式上线：2026 年 4 月起（含以后所有时间）
  const d = new Date()
  return d.getFullYear() > 2026 || (d.getFullYear() === 2026 && d.getMonth() >= 3)
}

// useBuddyNotification hook：
// - 仅在预告窗口内、且未孵化伴侣时触发
// - 在 Notification 区显示彩虹色 "/buddy" 文本（优先级 immediate，持续 15 秒）
// - RainbowText：每个字符用 getRainbowColor(i) 染色（来自 thinking.js 彩虹动画工具）
addNotification({
  key: 'buddy-teaser',
  jsx: <RainbowText text="/buddy" />,
  priority: 'immediate',
  timeoutMs: 15000,
})

// findBuddyTriggerPositions()：用于输入框中 /buddy 的语法高亮
// 正则：/\/buddy\b/g → 返回所有匹配位置 { start, end }
```

---

## 8. 配置存储

```json
// ~/.claude/config.json
{
  "companion": {
    "name": "Pebble",
    "personality": "calm and methodical, loves tracing bugs",
    "hatchedAt": 1710000000000
  },
  "companionMuted": false   // true = 静音（不显示精灵，不注入 intro）
}
// 注意：showBuddy 不存在 — 实际控制字段是 companionMuted（布尔反义）
// 注意：bones（稀有度/物种/帽子/stats）故意不存储！
```

---

## 9. 数据流全景

```
启动时：
  getCompanion()
      │
      ├── getGlobalConfig().companion → { name, personality, hatchedAt }
      │       └── 为空 → 未孵化
      │
      └── roll(companionUserId())
              │
              ├── companionUserId() = oauthAccount.accountUuid ?? userID ?? 'anon'
              ├── hash(userId + 'friend-2026-401')  [FNV-1a 或 Bun.hash]
              └── mulberry32(hash)                   [PRNG]
                      ↓ rollFrom()
                  { bones: { rarity, species, eye, hat, shiny, stats }, inspirationSeed }

渲染时（每 500ms）：
  CompanionSprite
      │
      ├── tick++（全局帧计数器）
      ├── step = IDLE_SEQUENCE[tick % 15]  → {0,1,2,-1}
      │       -1 → blink（eye→'-'）
      │       0-2 → 正常帧
      │
      ├── reaction?  → 兴奋模式（tick % frameCount 快速轮播）
      ├── petting?   → 爱心帧 prepend + 兴奋模式
      │
      ├── renderSprite(bones, spriteFrame)
      │       → {E} 替换为 bones.eye
      │       → 叠加帽子（line 0 为空时）
      │       → 删除多余空行
      │
      └── 布局：
          terminalColumns ≥ 100 → 完整精灵 + name + 气泡
          terminalColumns < 100 → renderFace() 单行降级

LLM 交互时：
  getCompanionIntroAttachment(messages)
      → 首次对话注入 companion_intro（告知 Claude 伴侣存在）
      → Claude 收到用户对伴侣说话 → defer 给气泡，自己退场
```

---

## 10. 设计亮点

### 10.1 无法作弊的稀有度
`CompanionBones`（含稀有度）**从不持久化**，每次从 `hash(userId + SALT)` 重算。即便直接编辑 `~/.claude/config.json` 写入 `"rarity": "legendary"`，下次启动也会被 `{ ...stored, ...bones }` 中的 `bones` 覆盖。

### 10.2 单槽缓存防止热路径重复计算
500ms 动画帧 + 每次按键 + 每轮对话共 3 条调用路径，全部使用相同 userId 调用 `roll()`。单槽 `rollCache` 以 `O(1)` 空间完美消除重复计算，无需引入完整 Map。

### 10.3 骨骼与灵魂分离的工程价值
- **骨骼**：外形，由算法决定，随代码更新自动同步（新增物种不破坏已有伴侣）
- **灵魂**：LLM 生成一次永久保存，代表用户与伴侣的独特关系
- 物种数组扩展后，老用户的伴侣可能"变种"——这是**有意为之**的机制，骨骼 roll 顺序不变，只是新物种加入后映射关系可能改变

### 10.4 双模式气泡渲染的工程决策
- **非全屏**：气泡 inline（PromptInput shrink）→ 稳定在 Ink 静态输出流中，随滚动历史保留
- **全屏**：气泡 float（BottomFloat slot）→ 避免 ScrollBox 的 overflowY:hidden 裁剪
- `companionReservedColumns()` 确保 PromptInput 精确知道可用列数

### 10.5 发布节奏的时区设计
彩虹预告通知使用**本地时间**（非 UTC），产生 24 小时滚动波，避免全球用户在同一 UTC 午夜同时看到 `/buddy` 彩虹提示，给 soul-gen（LLM 孵化接口）分摊负载。
