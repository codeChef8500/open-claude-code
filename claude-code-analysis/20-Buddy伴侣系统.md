# Claude Code 源码深度解读 — 20 Buddy 伴侣系统

> 覆盖文件：`src/buddy/`（companion.ts、types.ts、sprites.ts、CompanionSprite.tsx 45KB、useBuddyNotification.tsx 10KB、prompt.ts）

---

## 1. 模块职责概述

**Buddy（伴侣）系统**是 Claude Code 内置的数字宠物/伴侣功能。每个用户拥有一只独一无二的像素风格宠物，它在终端中以 ASCII 小精灵的形式存在，随着使用 Claude Code 而进化、成长。

**核心特点：**
- 外形由 `userId` 哈希**确定性生成**，无法伪造（换账号 = 换伙伴）
- 稀有度系统（common → legendary），传奇概率仅 1%
- 灵魂（名字 + 性格）由 **LLM 孵化**，第一次 `/hatch` 命令时生成
- ASCII sprite 在终端渲染，支持动画帧

---

## 2. 伴侣属性类型体系（`types.ts`）

### 2.1 物种（Species）
共 18 种物种，用 `String.fromCharCode` 编码避免字面量被代码扫描器识别：

```typescript
// duck, goose, blob, cat, dragon, octopus, owl, penguin,
// turtle, snail, ghost, axolotl, capybara, cactus, robot,
// rabbit, mushroom, chonk
export const SPECIES = [duck, goose, blob, cat, dragon, octopus, owl, penguin,
  turtle, snail, ghost, axolotl, capybara, cactus, robot, rabbit, mushroom, chonk]
```

### 2.2 稀有度（Rarity）
```typescript
// 权重（总和 = 100）
const RARITY_WEIGHTS = {
  common:    60,  // 60%
  uncommon:  25,  // 25%
  rare:      10,  // 10%
  epic:       4,  // 4%
  legendary:  1,  // 1%
}

// 稀有度视觉标识
const RARITY_STARS = {
  common:    '★',
  uncommon:  '★★',
  rare:      '★★★',
  epic:      '★★★★',
  legendary: '★★★★★',
}
```

### 2.3 帽子（Hat）
```typescript
const HATS = ['none', 'crown', 'tophat', 'propeller',
               'halo', 'wizard', 'beanie', 'tinyduck']
// common 稀有度的伴侣只有 'none'（无帽）
// uncommon+ 才有机会佩戴帽子
```

### 2.4 眼睛（Eye）
```typescript
const EYES = ['·', '✦', '×', '◉', '@', '°']
```

### 2.5 数值属性（Stats）
```typescript
const STAT_NAMES = ['DEBUGGING', 'PATIENCE', 'CHAOS', 'WISDOM', 'SNARK']

// 每个稀有度有不同的数值下限
const RARITY_FLOOR = {
  common: 5, uncommon: 15, rare: 25, epic: 35, legendary: 50,
}
// 特征：一个顶峰属性（+50）、一个垫底属性（-10）、其余随机
```

---

## 3. 伴侣生成算法（`companion.ts`）

### 3.1 确定性 PRNG（Mulberry32）
```typescript
// 轻量级种子伪随机数生成器（适合 hash→外形 的确定性生成）
function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return function () {
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}
```

### 3.2 从 userId 生成骨骼（Bones）
```typescript
const SALT = 'friend-2026-401'  // 盐值（防止枚举攻击）

export function roll(userId: string): Roll {
  const key = userId + SALT
  // LRU 缓存（防止高频渲染路径重复计算）
  if (rollCache?.key === key) return rollCache.value
  const value = rollFrom(mulberry32(hashString(key)))
  rollCache = { key, value }
  return value
}

function rollFrom(rng: () => number): Roll {
  const rarity = rollRarity(rng)    // 1. 决定稀有度
  const bones: CompanionBones = {
    rarity,
    species: pick(rng, SPECIES),    // 2. 选择物种
    eye:     pick(rng, EYES),       // 3. 眼睛类型
    hat: rarity === 'common' ? 'none' : pick(rng, HATS),  // 4. 帽子
    shiny: rng() < 0.01,            // 5. 1% 概率闪光版
    stats: rollStats(rng, rarity),  // 6. 数值
  }
  return { bones, inspirationSeed: Math.floor(rng() * 1e9) }
}
```

### 3.3 骨骼 vs 灵魂（数据分离设计）

```typescript
// 持久化存储（config.json）
type StoredCompanion = CompanionSoul & { hatchedAt: number }
// └── 只存 { name, personality, hatchedAt }，不存骨骼！

// 读取时动态重算骨骼（永远从 userId hash 来）
export function getCompanion(): Companion | undefined {
  const stored = getGlobalConfig().companion
  if (!stored) return undefined
  const { bones } = roll(companionUserId())
  // bones 覆盖 stored 中的旧格式字段
  return { ...stored, ...bones }  // Soul + fresh Bones
}
```

**设计意图：**
- 物种名称修改后，已有伴侣的外形自动更新（骨骼重算）
- 用户无法通过编辑 config.json 来伪造 legendary 稀有度
- 灵魂（名字/性格）由 LLM 生成后永久保存，体现"唯一性"

---

## 4. 孵化系统（Hatch）

首次使用 `/hatch` 命令时，LLM 会为伴侣生成独特的灵魂：

```typescript
// prompt.ts — 发送给 LLM 的孵化提示词
// 包含：物种、稀有度、眼睛、帽子、数值属性
// 要求 LLM 输出：
// {
//   name: string      // 独特的名字（2-3 个字符为佳）
//   personality: string  // 一句话性格描述
// }
```

`inspirationSeed`（从 Bones 生成的随机数）在孵化时传递给 LLM，确保同一伴侣每次孵化得到相同的灵感种子。

---

## 5. ASCII Sprite 渲染引擎（`CompanionSprite.tsx` 45KB）

这是最大的 Buddy 文件，包含所有物种的 ASCII 精灵图和动画帧：

```typescript
// 精灵图数据结构（sprites.ts）
type SpriteFrame = string[]  // 每行是一个字符串

type SpriteSet = {
  idle:   SpriteFrame[]   // 待机动画帧
  active: SpriteFrame[]   // 活跃动画帧（任务执行中）
  sleep:  SpriteFrame[]   // 睡眠动画帧
  shiny?: SpriteFrame[]   // 闪光版特殊帧
}

// 示例（duck 物种，idle 动画第 1 帧）：
// [
//   "  (·_·)  ",
//   " <( )   ",
//   "  / \   ",
// ]
```

### 5.1 帽子叠加（Hat Overlay）
```typescript
// 帽子被叠加在精灵图的第一行（head row）
// crown  → 在头部添加 "👑"
// tophat → 在头部添加 "🎩"
// wizard → 在头部添加 "🧙"
function applyHat(frame: SpriteFrame, hat: Hat): SpriteFrame {
  if (hat === 'none') return frame
  const hatGlyph = HAT_GLYPHS[hat]
  return [hatGlyph + frame[0], ...frame.slice(1)]
}
```

### 5.2 动画计时（500ms 帧率）
```typescript
// CompanionSprite.tsx 中使用 Ink 的 useInterval
// 每 500ms 切换一帧
const [frameIdx, setFrameIdx] = useState(0)
useInterval(() => {
  setFrameIdx(prev => (prev + 1) % frames.length)
}, 500)
```

### 5.3 状态驱动动画
```typescript
type CompanionState = 'idle' | 'active' | 'sleeping'

// Claude 正在执行任务 → active 帧
// Claude 空闲等待   → idle 帧
// SleepTool 调用中  → sleeping 帧
const frames = sprites[companion.species][state]
```

---

## 6. 伴侣通知系统（`useBuddyNotification.tsx` 10KB）

```typescript
// 当伴侣有新通知时，在输入框底部显示气泡
function useBuddyNotification({ companion, onDismiss }) {
  // 通知类型：
  // - 'level_up'    : 达成里程碑（使用次数、token 等）
  // - 'hatch'       : 首次孵化提示
  // - 'achievement' : 达成成就（如"连续使用 7 天"）
  // - 'stat_boost'  : 数值提升（稀有度升级时）
  
  useEffect(() => {
    const notification = checkPendingNotifications(companion)
    if (notification) {
      showBuddyNotification(notification)
    }
  }, [companion])
}
```

---

## 7. 伴侣展示位置

伴侣精灵图显示在 REPL.tsx 的底部输入区域旁边：

```tsx
// components/PromptInput/PromptInput.tsx 中
{companion && (
  <CompanionSprite
    companion={companion}
    state={isLoading ? 'active' : 'idle'}
    showBuddy={settings.showBuddy !== false}
  />
)}
```

---

## 8. 配置与显隐

```json
// ~/.claude/config.json
{
  "companion": {
    "name": "Pebble",
    "personality": "calm and methodical, loves tracing bugs",
    "hatchedAt": 1710000000000
  },
  "showBuddy": true   // 是否在 UI 中显示伴侣精灵
}
```

---

## 9. 设计哲学

### 9.1 无法作弊的稀有度
Bones（包括稀有度）从不持久化，每次启动从 `hash(userId + SALT)` 重算。用户无法通过修改配置文件来伪造 legendary 伙伴。

### 9.2 骨骼与灵魂分离
- **骨骼**：外形（物种/稀有度/帽子），由算法决定，代表"命运"
- **灵魂**：名字/性格，由 LLM 创造，代表"个性"
- 这种分离允许物种数据更新，同时保留用户珍视的名字/性格

### 9.3 服务于用户体验
伙伴系统不影响任何功能逻辑，是纯粹的 UI 层彩蛋，但通过"成就里程碑"激励用户持续使用 Claude Code，增加产品粘性。
