export default {
  "title": "竞猜后台",
  "description": "管理大小中竞猜配置，并查看回合、下注与账变审计记录",
  "tabs": {
    "settings": "配置",
    "rounds": "回合",
    "bets": "下注",
    "ledger": "账变",
    "stats": "统计"
  },
  "tabDescriptions": {
    "rounds": "查看每期概率、赔率与开奖结果快照",
    "bets": "审计玩家下注、结算与盈亏结果",
    "ledger": "核对下注扣款、派奖与退款流水",
    "stats": "按日期查看整体和用户盈亏统计"
  },
  "probabilitySummary": "当前概率：小 {small}% / 中 {mid}% / 大 {big}%",
  "oddsSummary": "当前赔率：小 {small}x / 中 {mid}x / 大 {big}x",
  "enabled": "启用活动",
  "enabledHint": "关闭后前台将进入维护状态，不再接受下注",
  "roundDuration": "每局时长（秒）",
  "betCloseOffset": "封盘时间（秒）",
  "allowedStakes": "允许积分",
  "customStakeMin": "自定义积分最小值",
  "customStakeMax": "自定义积分最大值",
  "allowedStakesPlaceholder": "例如：2, 5, 10, 20",
  "probabilitiesTitle": "概率配置",
  "oddsTitle": "赔率配置",
  "rulesMarkdown": "规则文案（Markdown）",
  "settingsLoadFailedTitle": "配置加载失败",
  "loadFailed": "加载竞猜后台数据失败",
  "saveSuccess": "竞猜配置已保存",
  "saveFailed": "保存竞猜配置失败",
  "invalidAllowedStakes": "请输入至少一个有效积分，多个值请用英文逗号分隔",
  "refundAction": "退款",
  "refundConfirmTitle": "确认退款",
  "refundConfirmMessage": "确认对第 {round} 期执行退款吗？",
  "refundSuccess": "退款完成，处理 {count} 条下注",
  "refundFailed": "退款失败",
  "filters": {
    "date": "日期",
    "roundId": "期号",
    "userId": "用户 ID",
    "status": "下注状态",
    "entryType": "流水类型"
  },
  "stats": {
    "participantCount": "参与人数",
    "totalStake": "总参与积分",
    "totalUserNet": "用户总盈亏",
    "houseNet": "系统总盈亏",
    "wonCount": "命中次数",
    "lostCount": "未命中次数",
    "refundedCount": "退款次数"
  },
  "validation": {
    "roundDuration": "每局时长必须大于 0",
    "betCloseOffset": "封盘时间必须在 0 到每局时长之间",
    "customStakeRange": "自定义积分范围不合法",
    "odds": "赔率必须大于 0"
  },
  "columns": {
    "round": "期号",
    "status": "状态",
    "result": "结果",
    "schedule": "时间窗口",
    "serverSeedHash": "种子哈希",
    "serverSeed": "服务端种子",
    "probabilities": "概率",
    "odds": "赔率",
    "user": "用户",
    "direction": "方向",
    "stake": "下注积分",
    "payout": "派奖",
    "net": "净结果",
    "createdAt": "时间",
    "entryType": "流水类型",
    "delta": "变动积分",
    "balanceWindow": "余额变化",
    "reason": "原因",
    "actions": "操作"
  },
  "status": {
    "open": "进行中",
    "settled": "已结算",
    "placed": "已下注",
    "won": "已中奖",
    "lost": "未中奖",
    "refunded": "已退款"
  },
  "entryType": {
    "bet_debit": "下注扣款",
    "bet_payout": "派奖入账",
    "bet_refund": "退款返还"
  }
}
