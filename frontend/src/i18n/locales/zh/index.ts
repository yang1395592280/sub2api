import wallet from './wallet'
import pelican from './pelican'
import customFeatures from './customFeatures'
import adminAccessKeys from './adminAccessKeys'
import mcp from './mcp'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import openAIQuality from './openAIQuality'
import misc from './misc'
import sizeBet from './sizeBet'
import gameCenter from './gameCenter'

export default {
  wallet,
  pelican,
  ...customFeatures,
  adminAccessKeys,
  mcp,
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...openAIQuality,
  sizeBet,
  gameCenter,
  nav: {
    ...common.nav,
    ...openAIQuality.nav,
    gameCenter: '游戏中心',
    gameCenterAdmin: '游戏中心管理',
    sizeBet: '猜大小'
  },
  ...misc,
}
