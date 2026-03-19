// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import sitemap from '@astrojs/sitemap';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  site: 'https://andrekurait.github.io',
  base: '/TemporalCI',
  vite: {
    plugins: [tailwindcss()],
  },
  integrations: [
    starlight({
      title: 'TemporalCI',
      description: 'Kubernetes-native CI powered by Temporal — durable, observable, replayable pipelines.',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/AndreKurait/TemporalCI' },
      ],
      sidebar: [
        {
          label: 'Start Here',
          items: [
            { label: 'Overview', slug: '' },
            { label: 'Quick Start', slug: 'getting-started/quickstart' },
            { label: 'Local Development', slug: 'getting-started/local-dev' },
          ],
        },
        {
          label: 'Configuration',
          items: [
            { label: 'Pipeline Config (.temporalci.yaml)', slug: 'configuration/pipeline' },
            { label: 'GitHub Secrets', slug: 'configuration/github-secrets' },
          ],
        },
        {
          label: 'Architecture',
          items: [
            { label: 'System Overview', slug: 'architecture/overview' },
            { label: 'Workflow Execution', slug: 'architecture/workflows' },
          ],
        },
        {
          label: 'Concepts',
          items: [
            { label: 'Why Temporal for CI?', slug: 'concepts/why-temporal' },
            { label: 'Pod Execution Model', slug: 'concepts/pod-execution' },
            { label: 'Security Model', slug: 'concepts/security' },
          ],
        },
        {
          label: 'Production',
          items: [
            { label: 'EKS Deployment', slug: 'production/eks-deployment' },
            { label: 'OIDC Bootstrap', slug: 'production/oidc-bootstrap' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Helm Values', slug: 'reference/helm-values' },
            { label: 'Environment Variables', slug: 'reference/env-vars' },
            { label: 'Troubleshooting', slug: 'reference/troubleshooting' },
          ],
        },
        {
          label: 'Roadmap',
          items: [
            { label: 'Roadmap', slug: 'roadmap' },
          ],
        },
      ],
      customCss: [
        '@fontsource/inter/400.css',
        '@fontsource/inter/500.css',
        '@fontsource/inter/600.css',
        '@fontsource/inter/700.css',
        '@fontsource-variable/jetbrains-mono/index.css',
        './src/styles/custom.css',
      ],
      head: [
        {
          tag: 'meta',
          attrs: { property: 'og:image', content: 'https://andrekurait.github.io/TemporalCI/og-image.png' },
        },
        {
          tag: 'meta',
          attrs: { property: 'og:type', content: 'website' },
        },
        {
          tag: 'meta',
          attrs: { name: 'twitter:card', content: 'summary_large_image' },
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/AndreKurait/TemporalCI/edit/main/site/',
      },
    }),
    sitemap(),
  ],
});
