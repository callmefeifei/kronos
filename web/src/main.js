import { createApp } from 'vue'
import { createPinia } from 'pinia'
// Element Plus CSS is auto-imported per-component by unplugin-vue-components.
// Only import the base/reset styles.
import 'element-plus/theme-chalk/base.css'

import App from './App.vue'
import router from './router'
import './assets/main.css'

// Only register icons used dynamically by string name (sidebar menu).
import { Odometer, List, User, Bell } from '@element-plus/icons-vue'

const app = createApp(App)

const dynamicIcons = { Odometer, List, User, Bell }
for (const [key, component] of Object.entries(dynamicIcons)) {
  app.component(key, component)
}

app.use(createPinia())
app.use(router)

app.mount('#app')
