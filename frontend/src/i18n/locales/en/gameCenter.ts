export default {
  hero: {
    tag: 'GAME CENTER',
    title: 'Game Center',
    subtitle: 'Current Points'
  },
  loadError: {
    title: 'Failed to Load Game Center',
    description: 'Unable to load game center data at this time. Please try again later.'
  },
  disabled: {
    title: 'Game Center Disabled',
    description: 'Game center has been closed by administrator.'
  },
  claim: {
    title: 'Daily Rewards',
    subtitle: 'Claim points reward by batch schedule',
    reward: 'Claim {points} points in this batch',
    action: 'Claim Now',
    claiming: 'Claiming...',
    success: 'Claimed successfully',
    failed: 'Failed to claim. Please try again later.',
    empty: 'No claimable batches available at the moment.',
    status: {
      pending: 'Upcoming',
      claimable: 'Claimable',
      claimed: 'Claimed'
    }
  },
  exchange: {
    entry: 'Points Exchange',
    title: 'Points Exchange',
    subtitle: 'Transfer between welfare balance and game points',
    balanceToPoints: 'Welfare Balance to Points',
    pointsToBalance: 'Points to Welfare Balance',
    rate: 'Rate: {rate}',
    amount: 'Balance Amount',
    points: 'Points Amount',
    submit: 'Submit Exchange',
    submitting: 'Exchanging...',
    success: 'Exchanged successfully',
    failed: 'Exchange failed. Please try again later.'
  },
  launch: {
    quick: 'Quick Start',
    fullscreen: 'Fullscreen'
  },
  catalog: {
    title: 'Games Catalog',
    subtitle: 'Select a mini-game to play',
    noSubtitle: 'Play instantly',
    empty: 'No games available',
    unsupportedGame: 'This game does not support quick start',
    gameTop10: 'Top 10 Players by Points',
    luckyWheelTop10: 'Top 10 Lucky Players Today',
    totalRemainingPoints: 'Points: {points}',
    todayLuckValue: 'Luck Value: {points}'
  },
  leaderboard: {
    title: 'Points Leaderboard',
    subtitle: 'View user points rankings. Click your entry to see details',
    empty: 'No leaderboard data',
    detailTitle: "Points History of {user}",
    detailPlaceholder: 'Points History',
    detailHint: 'Click yourself in leaderboard to view ledger. Admins can view any user.',
    ownOnly: 'Regular users can only view their own ledger'
  },
  embed: {
    title: 'Quick Play Window'
  },
  ledger: {
    title: 'My Points Ledger',
    empty: 'No points history yet',
    startDate: 'Start Date',
    endDate: 'End Date',
    types: {
      daily_claim: 'Daily Claim',
      exchange_from_balance: 'Balance to Points',
      exchange_to_balance: 'Points to Balance',
      exchange_from_welfare: 'Welfare Balance to Points',
      exchange_to_gift_balance: 'Points to Welfare Balance',
      admin_adjust: 'Admin Adjustment',
      redeem_code: 'Points Code Redemption',
      bet_debit: 'Game Bet Debit',
      bet_payout: 'Game Win Payout',
      bet_refund: 'Game Bet Refund',
      lucky_wheel_spin: 'Lucky Wheel'
    }
  },
  shell: {
    titlePrefix: 'Fullscreen Mode',
    back: 'Back to Game Center',
    openRaw: 'Open in New Tab',
    unsupportedTitle: 'Game Not Supported',
    unsupportedDescription: 'Please return to Game Center and select another game.'
  }
}
