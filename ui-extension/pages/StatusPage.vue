<script>
import { mapGetters } from 'vuex';
import { PRODUCT_NAME, PAGES } from '../product';

const MGMT_CLUSTER   = 'management.cattle.io.cluster';
const MGMT_PROJECT   = 'management.cattle.io.project';
const MGMT_PRTB      = 'management.cattle.io.projectroletemplatebinding';
const MGMT_CRTB      = 'management.cattle.io.clusterroletemplatebinding';
const CATALOG_REPO   = 'catalog.cattle.io.clusterrepo';

// Projects excluded from migration (Rancher defaults)
const SKIP_PROJECTS  = ['Default', 'System'];

// CRTBs excluded from migration (Rancher defaults)
const SKIP_CRTB_NAMES   = ['creator-cluster-owner'];
const SKIP_CRTB_PREFIXES = ['fleet-default-owner'];

// Repos excluded from migration (Rancher defaults)
const SKIP_REPO_PREFIX = 'rancher-';

export const STATUS = {
  MIGRATED:     'migrated',
  NOT_MIGRATED: 'not-migrated',
  DIFF:         'diff',
};

export default {
  name: 'CattleDriveStatus',

  async fetch() {
    const source = this.$route.query.source;
    const target = this.$route.query.target;

    if (!source || !target) {
      return;
    }

    this.sourceId = source;
    this.targetId = target;

    const allClusters = await this.$store.dispatch('management/findAll', { type: MGMT_CLUSTER });
    this.sourceCluster = allClusters.find(c => c.id === source);
    this.targetCluster = allClusters.find(c => c.id === target);

    if (!this.sourceCluster || !this.targetCluster) {
      this.fetchError = 'Could not find one or both clusters.';
      return;
    }

    await this.loadStatus();
  },

  data() {
    return {
      sourceId:      '',
      targetId:      '',
      sourceCluster: null,
      targetCluster: null,
      sections:      [],
      fetchError:    null,
      loading:       false,
    };
  },

  computed: {
    ...mapGetters({ t: 'i18n/t' }),

    sourceName() {
      return this.sourceCluster?.spec?.displayName || this.sourceId;
    },

    targetName() {
      return this.targetCluster?.spec?.displayName || this.targetId;
    },

    totalObjects() {
      return this.sections.reduce((sum, s) => sum + s.items.length, 0);
    },

    notMigratedCount() {
      return this.sections.reduce((sum, s) =>
        sum + s.items.filter(i => i.status === STATUS.NOT_MIGRATED).length, 0);
    },

    diffCount() {
      return this.sections.reduce((sum, s) =>
        sum + s.items.filter(i => i.status === STATUS.DIFF).length, 0);
    },
  },

  methods: {
    async loadStatus() {
      this.loading = true;
      this.fetchError = null;

      try {
        // Fetch all object types in parallel; the management store is shared so
        // filtering by clusterName afterwards is both correct and cache-friendly.
        const [
          allProjects,
          allPRTBs,
          allCRTBs,
          allRepos,
        ] = await Promise.all([
          this.$store.dispatch('management/findAll', { type: MGMT_PROJECT }),
          this.$store.dispatch('management/findAll', { type: MGMT_PRTB }),
          this.$store.dispatch('management/findAll', { type: MGMT_CRTB }),
          this.$store.dispatch('management/findAll', { type: CATALOG_REPO }),
        ]);

        // Split by cluster
        const sourceProjects = allProjects;
        const targetProjects = allProjects;
        const sourceCRTBs    = allCRTBs;
        const targetCRTBs    = allCRTBs;
        const sourceRepos    = allRepos;
        const targetRepos    = allRepos;

        // Filter by cluster
        const srcProjects = sourceProjects
          .filter(p => p.spec?.clusterName === this.sourceId && !SKIP_PROJECTS.includes(p.spec?.displayName));
        const tgtProjects = targetProjects
          .filter(p => p.spec?.clusterName === this.targetId && !SKIP_PROJECTS.includes(p.spec?.displayName));

        const srcCRTBs = sourceCRTBs
          .filter(c => c.clusterName === this.sourceId || c.metadata?.namespace === this.sourceId)
          .filter(c => !SKIP_CRTB_NAMES.includes(c.name) && !SKIP_CRTB_PREFIXES.some(p => c.name?.startsWith(p)));
        const tgtCRTBs = targetCRTBs
          .filter(c => c.clusterName === this.targetId || c.metadata?.namespace === this.targetId)
          .filter(c => !SKIP_CRTB_NAMES.includes(c.name) && !SKIP_CRTB_PREFIXES.some(p => c.name?.startsWith(p)));

        const srcRepos = sourceRepos
          .filter(r => !r.metadata?.name?.startsWith(SKIP_REPO_PREFIX));
        const tgtRepos = targetRepos
          .filter(r => !r.metadata?.name?.startsWith(SKIP_REPO_PREFIX));

        // Build project sections with nested PRTBs
        const projectItems = srcProjects.map((srcP) => {
          const tgtP = tgtProjects.find(p => p.spec?.displayName === srcP.spec?.displayName);
          const projectStatus = !tgtP ? STATUS.NOT_MIGRATED
            : this.specDiffers(srcP.spec, tgtP.spec) ? STATUS.DIFF
              : STATUS.MIGRATED;

          // PRTBs for this project
          const srcPRTBs = allPRTBs.filter(b => b.projectName === srcP.id || b.spec?.projectName === srcP.id);
          const tgtPRTBs = tgtP ? allPRTBs.filter(b => b.projectName === tgtP.id || b.spec?.projectName === tgtP.id) : [];

          const prtbItems = srcPRTBs
            .filter(b => !['creator-project-owner', 'creator-project-member'].includes(b.name))
            .map((srcB) => {
              const tgtB = tgtPRTBs.find(b => b.name === srcB.name);
              return {
                name:   srcB.name,
                label:  srcB.name,
                type:   'prtb',
                status: !tgtB ? STATUS.NOT_MIGRATED
                  : this.specDiffers(srcB, tgtB) ? STATUS.DIFF
                    : STATUS.MIGRATED,
              };
            });

          return {
            name:     srcP.spec?.displayName,
            label:    srcP.spec?.displayName,
            type:     'project',
            status:   projectStatus,
            children: prtbItems,
          };
        });

        // CRTB items
        const crtbItems = srcCRTBs.map((srcC) => {
          const tgtC = tgtCRTBs.find(c => c.name === srcC.name);
          return {
            name:   srcC.name,
            label:  srcC.name,
            type:   'crtb',
            status: !tgtC ? STATUS.NOT_MIGRATED
              : this.specDiffers(srcC, tgtC) ? STATUS.DIFF
                : STATUS.MIGRATED,
          };
        });

        // Catalog repo items
        const repoItems = srcRepos.map((srcR) => {
          const tgtR = tgtRepos.find(r => r.name === srcR.name || r.metadata?.name === srcR.metadata?.name);
          return {
            name:   srcR.metadata?.name || srcR.name,
            label:  srcR.metadata?.name || srcR.name,
            type:   'repo',
            status: !tgtR ? STATUS.NOT_MIGRATED
              : this.specDiffers(srcR.spec, tgtR.spec) ? STATUS.DIFF
                : STATUS.MIGRATED,
          };
        });

        this.sections = [
          { title: 'Projects', icon: 'folder', items: projectItems },
          { title: 'Cluster Role Bindings', icon: 'user', items: crtbItems },
          { title: 'Catalog Repos', icon: 'catalog', items: repoItems },
        ];
      } catch (err) {
        this.fetchError = err?.message || String(err);
      } finally {
        this.loading = false;
      }
    },

    specDiffers(a, b) {
      // Simple deep equality check (Vue's JSON comparison)
      return JSON.stringify(a) !== JSON.stringify(b);
    },

    statusLabel(status) {
      switch (status) {
      case STATUS.MIGRATED:     return 'Migrated';
      case STATUS.NOT_MIGRATED: return 'Not Migrated';
      case STATUS.DIFF:         return 'Drift Detected';
      default:                  return 'Unknown';
      }
    },

    statusColor(status) {
      switch (status) {
      case STATUS.MIGRATED:     return 'success';
      case STATUS.NOT_MIGRATED: return 'error';
      case STATUS.DIFF:         return 'warning';
      default:                  return 'info';
      }
    },

    goToDashboard() {
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.DASHBOARD }`,
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },

    goToMigrate() {
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.MIGRATE }`,
        query:  { source: this.sourceId, target: this.targetId },
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },
  },
};
</script>

<template>
  <div class="cattle-drive-status">
    <!-- Header -->
    <div class="page-header">
      <div class="page-header__left">
        <button class="btn btn-sm role-link" @click="goToDashboard">
          <i class="icon icon-chevron-left" /> Back
        </button>
        <h1>Migration Status</h1>
      </div>
      <div class="page-header__right">
        <button
          class="btn role-secondary mr-10"
          :disabled="loading"
          @click="loadStatus"
        >
          <i class="icon icon-refresh" /> Refresh
        </button>
        <button
          class="btn role-primary"
          :disabled="notMigratedCount === 0 && diffCount === 0"
          @click="goToMigrate"
        >
          <i class="icon icon-upload" /> Run Migration
        </button>
      </div>
    </div>

    <!-- Cluster label strip -->
    <div class="cluster-strip">
      <span class="cluster-strip__item">
        <i class="icon icon-server" />
        <strong>Source:</strong> {{ sourceName }}
      </span>
      <i class="icon icon-chevron-right cluster-strip__arrow" />
      <span class="cluster-strip__item">
        <i class="icon icon-server" />
        <strong>Target:</strong> {{ targetName }}
      </span>
    </div>

    <!-- Summary badges -->
    <div v-if="!loading && !fetchError" class="summary-row mt-20">
      <div class="summary-badge summary-badge--total">
        <span class="summary-badge__count">{{ totalObjects }}</span>
        <span class="summary-badge__label">Total Objects</span>
      </div>
      <div class="summary-badge summary-badge--success">
        <span class="summary-badge__count">{{ totalObjects - notMigratedCount - diffCount }}</span>
        <span class="summary-badge__label">Migrated</span>
      </div>
      <div class="summary-badge summary-badge--error">
        <span class="summary-badge__count">{{ notMigratedCount }}</span>
        <span class="summary-badge__label">Not Migrated</span>
      </div>
      <div class="summary-badge summary-badge--warning">
        <span class="summary-badge__count">{{ diffCount }}</span>
        <span class="summary-badge__label">Drift Detected</span>
      </div>
    </div>

    <!-- Loading -->
    <Loading v-if="loading" />

    <!-- Error -->
    <Banner v-else-if="fetchError" color="error" :label="fetchError" />

    <!-- No query params -->
    <Banner
      v-else-if="!sourceId || !targetId"
      color="warning"
      label="No clusters selected. Please go back to the dashboard and select source and target clusters."
    />

    <!-- Status tree -->
    <div v-else class="status-tree mt-20">
      <div
        v-for="section in sections"
        :key="section.title"
        class="status-section"
      >
        <div class="status-section__header">
          <i :class="`icon icon-${ section.icon }`" />
          {{ section.title }}
          <span class="status-section__count">({{ section.items.length }})</span>
        </div>

        <div v-if="section.items.length === 0" class="status-section__empty">
          No objects found.
        </div>

        <div
          v-for="item in section.items"
          :key="item.name"
          class="status-item"
        >
          <div class="status-item__row">
            <span class="status-item__name">{{ item.label }}</span>
            <span :class="`badge badge--${ statusColor(item.status) }`">
              {{ statusLabel(item.status) }}
            </span>
          </div>

          <!-- Nested children (PRTBs inside a project) -->
          <div
            v-if="item.children && item.children.length > 0"
            class="status-item__children"
          >
            <div
              v-for="child in item.children"
              :key="child.name"
              class="status-item status-item--child"
            >
              <div class="status-item__row">
                <span class="status-item__name">
                  <i class="icon icon-user mr-5" />{{ child.label }}
                </span>
                <span :class="`badge badge--${ statusColor(child.status) }`">
                  {{ statusLabel(child.status) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.cattle-drive-status {
  padding: 20px;

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--border);
    padding-bottom: 15px;
    margin-bottom: 15px;

    &__left {
      display: flex;
      align-items: center;
      gap: 10px;

      h1 {
        font-size: 22px;
        font-weight: 600;
        margin: 0;
      }
    }

    &__right {
      display: flex;
      align-items: center;
    }
  }

  .cluster-strip {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 15px;
    background: var(--box-bg);
    border: 1px solid var(--border);
    border-radius: var(--border-radius);
    font-size: 14px;

    &__item {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    &__arrow {
      color: var(--primary);
    }
  }

  .summary-row {
    display: flex;
    gap: 12px;
  }

  .summary-badge {
    flex: 1;
    padding: 14px 16px;
    border-radius: var(--border-radius);
    border: 1px solid var(--border);
    text-align: center;

    &__count {
      display: block;
      font-size: 28px;
      font-weight: 700;
    }

    &__label {
      display: block;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      margin-top: 4px;
    }

    &--total   { background: var(--box-bg); }
    &--success { background: var(--success-banner-bg); color: var(--success); }
    &--error   { background: var(--error-banner-bg); color: var(--error); }
    &--warning { background: var(--warning-banner-bg); color: var(--warning); }
  }

  .status-tree {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .status-section {
    border: 1px solid var(--border);
    border-radius: var(--border-radius);
    overflow: hidden;

    &__header {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 10px 16px;
      background: var(--box-bg);
      font-weight: 600;
      font-size: 14px;
      border-bottom: 1px solid var(--border);
    }

    &__count {
      color: var(--body-text);
      font-weight: 400;
    }

    &__empty {
      padding: 12px 16px;
      color: var(--body-text);
      font-style: italic;
      font-size: 13px;
    }
  }

  .status-item {
    border-bottom: 1px solid var(--border);

    &:last-child {
      border-bottom: none;
    }

    &--child {
      background: var(--body-bg);
    }

    &__row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 10px 16px;
    }

    &__name {
      font-size: 13px;
    }

    &__children {
      border-top: 1px solid var(--border);
      margin-left: 24px;
    }
  }

  .badge {
    font-size: 11px;
    font-weight: 600;
    padding: 3px 8px;
    border-radius: 12px;
    text-transform: uppercase;
    letter-spacing: 0.04em;

    &--success { background: var(--success-banner-bg); color: var(--success); }
    &--error   { background: var(--error-banner-bg);   color: var(--error); }
    &--warning { background: var(--warning-banner-bg); color: var(--warning); }
    &--info    { background: var(--info-banner-bg);    color: var(--info); }
  }
}
</style>
