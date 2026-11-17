export default {
  title: 'Game Center Management',
  description: 'Manage daily claims, points exchanges, and game catalog settings.',
  sections: {
    claim: 'Daily Claim Schedule',
    exchange: 'Exchange Settings',
    catalog: 'Games Catalog',
    operations: 'Manual Points Adjustment',
    audit: 'Audit & Records'
  },
  switches: {
    gameCenterEnabled: 'Enable Game Center',
    gameCenterEnabledHint: 'When disabled, game center entry is hidden from users',
    claimEnabled: 'Enable Daily Claims',
    claimEnabledHint: 'When disabled, users cannot claim scheduled points'
  },
  claim: {
    batchKey: 'Batch Key',
    claimTime: 'Claim Time',
    pointsAmount: 'Points Amount',
    enabled: 'Enabled'
  },
  exchange: {
    balanceToPoints: 'Welfare Balance to Points',
    pointsToBalance: 'Points to Welfare Balance',
    rate: 'Exchange Rate',
    minAmount: 'Min Amount'
  },
  catalog: {
    empty: 'No games configured',
    sortOrder: 'Sort Order',
    openMode: 'Open Mode',
    openGameSettings: 'Game Settings',
    supportsEmbed: 'In-app Modal',
    supportsStandalone: 'Fullscreen',
    openModes: {
      dual: 'Dual Mode',
      embed: 'In-app Mode',
      standalone: 'Fullscreen'
    }
  },
  actions: {
    addBatch: 'Add Batch'
  },
  operations: {
    userId: 'User ID',
    user: 'User',
    userSearchPlaceholder: 'Search by email or username',
    selectedUser: 'Selected: {user}',
    deltaPoints: 'Points Adjustment (+/-)',
    reason: 'Reason',
    reasonPlaceholder: 'Optional reason',
    submit: 'Submit Adjustment',
    validation: 'Please select a user and enter adjustment amount',
    success: 'Points adjusted successfully',
    failed: 'Failed to adjust points'
  },
  audit: {
    description: 'Inspect points ledgers, claim records, and exchange history',
    ledger: 'Points Ledger',
    claims: 'Claim History',
    exchanges: 'Exchange History',
    startDate: 'Start Date',
    endDate: 'End Date',
    empty: 'No records found'
  },
  loadFailed: 'Failed to load game center configuration',
  saveSettingsSuccess: 'Game center settings saved',
  saveSettingsFailed: 'Failed to save game center settings',
  saveCatalogSuccess: 'Catalog configuration saved',
  saveCatalogFailed: 'Failed to save catalog configuration'
}
