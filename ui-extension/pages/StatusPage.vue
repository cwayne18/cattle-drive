<script>
import { PRODUCT_NAME, PAGES } from '../product';

const STATUS = {
  MIGRATED:     'migrated',
  NOT_MIGRATED: 'not-migrated',
  DIFF:         'diff',
};

export default {
  name: 'CattleDriveStatus',

  async fetch() {
    const q = this.$route.query;
    this.sourceId      = q.source     || '';
    this.targetId      = q.target     || '';
    this.apiBase       = q.apiBase    || '/k8s/clusters/local/api/v1/namespaces/cattle-system/services/http:cattle-drive-api:8080/proxy';
    this.kubeconfigPath = q.kubeconfig || '';

    if (this.sourceId && this.targetId) {
      await this.loadStatus();
    }
  },

  data() {
    return {
      sourceId:       '',
      targetId:       '',
      apiBase:        '/k8s/clusters/local/api/v1/namespaces/cattle-system/services/http:cattle-drive-api:8080/proxy',
      kubeconfigPath: '',
      sections:       [],
      fetchError:     null,
      loading:        false,
    };
  },

  computed: {
    totalObjects() {
      return this.sections.reduce((sum, s) =>
        sum + s.items.reduce((n, i) => n + 1 + (i.children ? i.children.length : 0), 0), 0);
    },

    notMigratedCount() {
      return this.sections.reduce((sum, s) =>
        sum + s.items.reduce((n, i) => {
          const childCount = (i.children || []).filter(c => !c.migrated).length;
          return n + (!i.migrated ? 1 : 0) + childCount;
        }, 0), 0);
    },

    diffCount() {
      return this.sections.reduce((sum, s) =>
        sum + s.items.reduce((n, i) => {
          const childCount = (i.children || []).filter(c => c.diff).length;
          return n + (i.diff ? 1 : 0) + childCount;
        }, 0), 0);
    },

    migratedCount() {
      return this.totalObjects - this.notMigratedCount - this.diffCount;
    },

    hasUnmigratedObjects() {
      return this.notMigratedCount > 0 || this.diffCount > 0;
    },
  },

  methods: {
    authHeaders() {
      const headers = { 'Content-Type': 'application/json' };
      const rawToken = this.$store?.getters?.['auth/token'];
      const token = typeof rawToken === 'string' ? rawToken : (rawToken?.token || rawToken?.value);
      if (token) {
        headers.Authorization = `Bearer ${ token }`;
      }
      return headers;
    },

    async loadStatus() {
      this.loading = true;
      this.fetchError = null;

      try {
        const body = {
          source: this.sourceId,
          target: this.targetId,
        };
        if (this.kubeconfigPath) {
          body.kubeconfig = this.kubeconfigPath;
        }
        const res = await fetch(`${ this.apiBase }/api/status`, {
          method:  'POST',
          credentials: 'same-origin',
          headers: this.authHeaders(),
          body:    JSON.stringify(body),
        });
        const data = await res.json();
        if (!res.ok) {
          throw new Error(data.error || `HTTP ${ res.status }`);
        }
        this.sections = this.buildSections(data);
      } catch (err) {
        this.fetchError = err.message || String(err);
      } finally {
        this.loading = false;
      }
    },

    buildSections(data) {
      // Projects (with nested PRTBs and namespaces as children)
      const projectItems = (data.projects || []).map(p => ({
        name:     p.name,
        label:    p.name,
        type:     'project',
        migrated: p.migrated,
        diff:     p.diff,
        children: [
          ...(p.prtbs || []).map(b => ({
            name:     b.name,
            label:    b.description ? `${ b.name }: ${ b.description }` : b.name,
            type:     'prtb',
            migrated: b.migrated,
            diff:     b.diff,
          })),
          ...(p.namespaces || []).map(ns => ({
            name:     ns.name,
            label:    ns.name,
            type:     'namespace',
            migrated: ns.migrated,
            diff:     ns.diff,
          })),
        ],
      }));

      const crtbItems = (data.clusterRoleBindings || []).map(c => ({
        name:     c.name,
        label:    c.description ? `${ c.name }: ${ c.description }` : c.name,
        type:     'crtb',
        migrated: c.migrated,
        diff:     c.diff,
      }));

      const repoItems = (data.catalogRepos || []).map(r => ({
        name:     r.name,
        label:    r.name,
        type:     'repo',
        migrated: r.migrated,
        diff:     r.diff,
      }));

      return [
        { title: 'Projects', icon: 'folder', items: projectItems },
        { title: 'Cluster Role Bindings', icon: 'user', items: crtbItems },
        { title: 'Catalog Repos', icon: 'catalog', items: repoItems },
      ];
    },

    itemStatus(item) {
      if (!item.migrated) return STATUS.NOT_MIGRATED;
      if (item.diff)      return STATUS.DIFF;
      return STATUS.MIGRATED;
    },

    childIcon(type) {
      const icons = {
        namespace: 'folder',
        prtb:      'user',
        crtb:      'user',
        repo:      'catalog',
      };
      return icons[type] || 'list-flat';
    },

    statusLabel(item) {
      switch (this.itemStatus(item)) {
      case STATUS.MIGRATED:     return 'Migrated';
      case STATUS.NOT_MIGRATED: return 'Not Migrated';
      case STATUS.DIFF:         return 'Drift Detected';
      default:                  return 'Unknown';
      }
    },

    statusColor(item) {
      switch (this.itemStatus(item)) {
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
        query:  {
          source:     this.sourceId,
          target:     this.targetId,
          apiBase:    this.apiBase,
          kubeconfig: this.kubeconfigPath,
        },
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
          :disabled="!hasUnmigratedObjects"
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
        <strong>Source:</strong> {{ sourceId }}
      </span>
      <i class="icon icon-chevron-right cluster-strip__arrow" />
      <span class="cluster-strip__item">
        <i class="icon icon-server" />
        <strong>Target:</strong> {{ targetId }}
      </span>
    </div>

    <!-- Summary badges -->
    <div v-if="!loading && !fetchError && sections.length > 0" class="summary-row mt-20">
      <div class="summary-badge summary-badge--total">
        <span class="summary-badge__count">{{ totalObjects }}</span>
        <span class="summary-badge__label">Total Objects</span>
      </div>
      <div class="summary-badge summary-badge--success">
        <span class="summary-badge__count">{{ migratedCount }}</span>
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
    <Banner v-else-if="fetchError" color="error" :label="fetchError" class="mt-20" />

    <!-- No query params -->
    <Banner
      v-else-if="!sourceId || !targetId"
      color="warning"
      label="No clusters selected. Please go back to the dashboard and select source/target clusters."
      class="mt-20"
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
            <span :class="`badge badge--${ statusColor(item) }`">
              {{ statusLabel(item) }}
            </span>
          </div>

          <!-- Nested children (PRTBs / namespaces inside a project) -->
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
                  <i :class="`icon icon-${ childIcon(child.type) } mr-5`" />
                  {{ child.label }}
                </span>
                <span :class="`badge badge--${ statusColor(child) }`">
                  {{ statusLabel(child) }}
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
