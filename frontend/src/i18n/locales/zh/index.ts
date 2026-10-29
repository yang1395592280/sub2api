import pelican from './pelican'
import customFeatures from './customFeatures'
import adminAccessKeys from './adminAccessKeys'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import openAIQuality from './openAIQuality'
import misc from './misc'

export default {
  pelican,
  ...customFeatures,
  adminAccessKeys,
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...openAIQuality,
  nav: { ...common.nav, ...openAIQuality.nav },
  ...misc,
}
