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
            { label: 'What is TemporalCI?', slug: 'getting-started/what-is-temporalci' },
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
        {
          label: 'Blog',
          items: [
            { label: 'All Posts', slug: 'blog' },
            { label: 'Why We Built CI on Temporal', slug: 'blog/why-temporal-for-ci' },
          ],
        },
      ],
      customCss: [
        './src/styles/custom.css',
      ],
      head: [
        {
          tag: 'link',
          attrs: {
            rel: 'preconnect',
            href: 'https://fonts.googleapis.com',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'preconnect',
            href: 'https://fonts.gstatic.com',
            crossorigin: true,
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'stylesheet',
            href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;700&display=swap',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'stylesheet',
            href: 'https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/css/all.min.css',
            integrity: 'sha512-Evv84Mr4kqVGRNSgIGL/F/aIDqQb7xQ2vcrdIwxfjThSH8CSR7PBEakCr51Ck+w+/U6swU2Im1vVX0SVk9ABhg==',
            crossorigin: 'anonymous',
            referrerpolicy: 'no-referrer',
          },
        },
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
        {
          tag: 'script',
          attrs: { type: 'application/ld+json' },
          content: JSON.stringify({
            '@context': 'https://schema.org',
            '@type': 'SoftwareApplication',
            name: 'TemporalCI',
            description: 'Kubernetes-native CI system built on Temporal for durable, observable, replayable pipelines.',
            url: 'https://andrekurait.github.io/TemporalCI/',
            applicationCategory: 'DeveloperApplication',
            operatingSystem: 'Kubernetes',
            license: 'https://github.com/AndreKurait/TemporalCI/blob/main/LICENSE',
            codeRepository: 'https://github.com/AndreKurait/TemporalCI',
            programmingLanguage: 'Go',
          }),
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/AndreKurait/TemporalCI/edit/main/site/',
      },
    }),
    sitemap(),
  ],
});
