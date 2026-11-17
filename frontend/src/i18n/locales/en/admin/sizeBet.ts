export default {
  "title": "Size Bet Admin",
  "description": "Manage size-bet settings and review rounds, bets, and ledger audit data",
  "tabs": {
    "settings": "Settings",
    "rounds": "Rounds",
    "bets": "Bets",
    "ledger": "Ledger",
    "stats": "Stats"
  },
  "tabDescriptions": {
    "rounds": "Review per-round probability, odds, and result snapshots",
    "bets": "Audit player bets, settlements, and net outcomes",
    "ledger": "Check debit, payout, and refund balance movements",
    "stats": "Review daily user and house profit statistics"
  },
  "probabilitySummary": "Current probability: Small {small}% / Mid {mid}% / Big {big}%",
  "oddsSummary": "Current odds: Small {small}x / Mid {mid}x / Big {big}x",
  "enabled": "Enable game",
  "enabledHint": "When disabled, the player page will switch to maintenance mode",
  "roundDuration": "Round duration (seconds)",
  "betCloseOffset": "Bet close offset (seconds)",
  "allowedStakes": "Allowed stakes",
  "customStakeMin": "Custom stake minimum",
  "customStakeMax": "Custom stake maximum",
  "allowedStakesPlaceholder": "Example: 2, 5, 10, 20",
  "probabilitiesTitle": "Probability settings",
  "oddsTitle": "Odds settings",
  "rulesMarkdown": "Rules markdown",
  "settingsLoadFailedTitle": "Failed to load settings",
  "loadFailed": "Failed to load size-bet admin data",
  "saveSuccess": "Size-bet settings saved",
  "saveFailed": "Failed to save size-bet settings",
  "invalidAllowedStakes": "Enter at least one valid stake and separate multiple values with commas",
  "refundAction": "Refund",
  "refundConfirmTitle": "Confirm refund",
  "refundConfirmMessage": "Refund round {round}?",
  "refundSuccess": "Refund completed for {count} bets",
  "refundFailed": "Refund failed",
  "filters": {
    "date": "Date",
    "roundId": "Round",
    "userId": "User ID",
    "status": "Bet Status",
    "entryType": "Entry Type"
  },
  "stats": {
    "participantCount": "Participants",
    "totalStake": "Total Stake",
    "totalUserNet": "User Net",
    "houseNet": "House Net",
    "wonCount": "Wins",
    "lostCount": "Losses",
    "refundedCount": "Refunds"
  },
  "validation": {
    "roundDuration": "Round duration must be greater than 0",
    "betCloseOffset": "Bet close offset must be between 0 and the round duration",
    "customStakeRange": "Custom stake range is invalid",
    "odds": "Odds must be greater than 0"
  },
  "columns": {
    "round": "Round",
    "status": "Status",
    "result": "Result",
    "schedule": "Schedule",
    "serverSeedHash": "Seed Hash",
    "serverSeed": "Server Seed",
    "probabilities": "Probability",
    "odds": "Odds",
    "user": "User",
    "direction": "Direction",
    "stake": "Stake",
    "payout": "Payout",
    "net": "Net",
    "createdAt": "Created At",
    "entryType": "Entry Type",
    "delta": "Delta",
    "balanceWindow": "Balance Change",
    "reason": "Reason",
    "actions": "Actions"
  },
  "status": {
    "open": "Open",
    "settled": "Settled",
    "placed": "Placed",
    "won": "Won",
    "lost": "Lost",
    "refunded": "Refunded"
  },
  "entryType": {
    "bet_debit": "Bet Debit",
    "bet_payout": "Bet Payout",
    "bet_refund": "Bet Refund"
  }
}
