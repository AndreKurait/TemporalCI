// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://andrekurait.github.io',
  base: '/TemporalCI',
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
          label: 'Production',
          items: [
            { label: 'EKS Deployment', slug: 'production/eks-deployment' },
            { label: 'OIDC Bootstrap', slug: 'production/oidc-bootstrap' },
            { label: 'Auto Mode Migration', slug: 'production/automode-migration' },
          ],
        },
        {
          label: 'Roadmap',
          items: [
            { label: 'Roadmap', slug: 'roadmap' },
          ],
        },
      ],
      customCss: ['./src/styles/custom.css'],
      head: [
        {
          tag: 'meta',
          attrs: { property: 'og:image', content: 'https://andrekurait.github.io/TemporalCI/og-image.png' },
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/AndreKurait/TemporalCI/edit/main/site/',
      },
    }),
  ],
});
