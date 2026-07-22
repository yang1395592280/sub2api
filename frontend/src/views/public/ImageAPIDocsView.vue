<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white/95 dark:border-dark-800 dark:bg-dark-900/95">
      <div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-3">
          <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700">
            <img src="/logo.svg" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-base font-semibold text-gray-950 dark:text-white">Sub2API</span>
        </RouterLink>
        <RouterLink
          to="/login"
          class="inline-flex flex-shrink-0 items-center justify-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/20 transition hover:bg-primary-700"
        >
          {{ t('home.login') }}
        </RouterLink>
      </div>
    </header>

    <main class="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:py-10">
      <section class="mb-8 border-b border-gray-200 pb-6 dark:border-dark-700">
        <p class="text-sm font-medium text-primary-700 dark:text-primary-300">{{ t('imageApiDocs.subtitle') }}</p>
        <h1 class="mt-2 text-3xl font-bold tracking-normal text-gray-950 dark:text-white sm:text-4xl">
          {{ t('imageApiDocs.title') }}
        </h1>
      </section>

      <article class="space-y-8">
        <section class="doc-section">
          <h2>{{ t('imageApiDocs.baseUrl') }}</h2>
          <CodeBlock code="https://www.loomex.site/v1" />
        </section>

        <section class="doc-section">
          <h2>{{ t('imageApiDocs.auth') }}</h2>
          <p>所有请求都需要在 Header 中携带 API Key。</p>
          <CodeBlock code="Authorization: Bearer YOUR_API_KEY&#10;Content-Type: application/json" />
        </section>

        <section class="doc-section">
          <h2>{{ t('imageApiDocs.textToImage') }}</h2>
          <p><code>POST /images/generations</code></p>
          <CodeBlock :code="generationExample" />
          <CodeBlock :code="generationResponseExample" />
        </section>

        <section class="doc-section">
          <h2>{{ t('imageApiDocs.imageEdit') }}</h2>
          <p><code>POST /images/edits</code></p>
          <CodeBlock :code="editExample" />
        </section>

        <section class="doc-section">
          <h2>{{ t('imageApiDocs.params') }}</h2>
          <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left font-semibold">参数</th>
                  <th class="px-4 py-3 text-left font-semibold">类型</th>
                  <th class="px-4 py-3 text-left font-semibold">说明</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-900">
                <tr v-for="param in params" :key="param.name">
                  <td class="px-4 py-3 font-mono text-primary-700 dark:text-primary-300">{{ param.name }}</td>
                  <td class="px-4 py-3 text-gray-600 dark:text-dark-300">{{ param.type }}</td>
                  <td class="px-4 py-3 text-gray-700 dark:text-dark-200">{{ param.desc }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="doc-section">
          <h2>{{ t('imageApiDocs.errors') }}</h2>
          <CodeBlock :code="errorExample" />
        </section>
      </article>
    </main>
  </div>
</template>

<script setup lang="ts">
import { defineComponent, h } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const CodeBlock = defineComponent({
  name: 'CodeBlock',
  props: {
    code: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    return () => h('pre', { class: 'doc-code' }, [h('code', props.code)])
  },
})

const generationExample = `curl https://www.loomex.site/v1/images/generations \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只橘猫坐在未来城市的窗边",
    "size": "1024x1024",
    "quality": "high",
    "output_format": "png",
    "response_format": "url"
  }'`

const generationResponseExample = `{
  "created": 1710000007,
  "model": "gpt-image-2",
  "data": [
    {
      "url": "https://www.loomex.site/v1/images/files/abc123.png"
    }
  ]
}`

const editExample = `curl https://www.loomex.site/v1/images/edits \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "把背景替换为夜晚的海边",
    "images": [
      {
        "image_url": "https://www.loomex.site/example/source.png"
      }
    ],
    "size": "1024x1024",
    "response_format": "url"
  }'`

const errorExample = `{
  "error": {
    "message": "Image generation failed",
    "type": "image_generation_failed",
    "code": "image_generation_failed"
  }
}`

const params = [
  { name: 'model', type: 'string', desc: '图片模型，推荐 gpt-image-2。' },
  { name: 'prompt', type: 'string', desc: '图片描述或编辑指令。' },
  { name: 'size', type: 'string', desc: '1024x1024、1824x1024、1024x1824、2048x2048、3840x2160、2160x3840。' },
  { name: 'response_format', type: 'string', desc: 'url 或 b64_json，默认建议使用 url。' },
  { name: 'output_format', type: 'string', desc: '输出格式：png、jpeg、webp。' },
  { name: 'quality', type: 'string', desc: '输出质量，例如 auto、medium、high。' },
]
</script>

<style scoped>
.doc-section {
  @apply rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-900;
}

.doc-section h2 {
  @apply mb-3 text-xl font-bold tracking-normal text-gray-950 dark:text-white;
}

.doc-section p {
  @apply mb-4 text-sm leading-6 text-gray-700 dark:text-dark-200;
}

.doc-section code {
  @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-sm text-gray-900 dark:bg-dark-800 dark:text-dark-100;
}

.doc-code {
  @apply mb-4 overflow-x-auto whitespace-pre-wrap break-words rounded-lg border border-gray-200 bg-gray-950 p-4 text-sm leading-6 text-gray-100 dark:border-dark-700;
}

.doc-code code {
  @apply bg-transparent p-0 text-gray-100;
}
</style>
