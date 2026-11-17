export default {
  "title": "游戏中心管理",
  "description": "统一配置每日领取、积分兑换与游戏目录开关。",
  "sections": {
    "claim": "每日领取配置",
    "exchange": "兑换配置",
    "catalog": "游戏目录配置",
    "operations": "手动积分调整",
    "audit": "运营查询"
  },
  "switches": {
    "gameCenterEnabled": "启用游戏中心",
    "gameCenterEnabledHint": "关闭后用户端隐藏游戏中心入口与页面",
    "claimEnabled": "启用每日领取",
    "claimEnabledHint": "关闭后用户无法在领取时段领取积分"
  },
  "claim": {
    "batchKey": "批次标识",
    "claimTime": "领取时间",
    "pointsAmount": "积分数量",
    "enabled": "启用"
  },
  "exchange": {
    "balanceToPoints": "余额兑积分",
    "pointsToBalance": "积分兑余额",
    "rate": "兑换倍率",
    "minAmount": "最小兑换额度"
  },
  "catalog": {
    "empty": "暂无可配置游戏",
    "sortOrder": "排序",
    "openMode": "打开方式",
    "openGameSettings": "进入游戏配置",
    "supportsEmbed": "站内打开",
    "supportsStandalone": "全屏打开",
    "openModes": {
      "dual": "双模式",
      "embed": "站内模式",
      "standalone": "全屏模式"
    }
  },
  "actions": {
    "addBatch": "新增批次"
  },
  "operations": {
    "userId": "用户 ID",
    "user": "用户",
    "userSearchPlaceholder": "输入邮箱或用户名搜索",
    "selectedUser": "已选择：{user}",
    "deltaPoints": "调整积分",
    "reason": "原因",
    "reasonPlaceholder": "可不填写",
    "submit": "提交调整",
    "validation": "请选择用户并填写调整积分",
    "success": "积分调整成功",
    "failed": "积分调整失败"
  },
  "audit": {
    "description": "查看最近积分流水、领取记录和兑换记录",
    "ledger": "积分流水",
    "claims": "领取记录",
    "exchanges": "兑换记录",
    "startDate": "开始日期",
    "endDate": "结束日期",
    "empty": "暂无记录"
  },
  "loadFailed": "加载游戏中心配置失败",
  "saveSettingsSuccess": "游戏中心配置已保存",
  "saveSettingsFailed": "保存游戏中心配置失败",
  "saveCatalogSuccess": "目录配置已保存",
  "saveCatalogFailed": "保存目录配置失败"
}
