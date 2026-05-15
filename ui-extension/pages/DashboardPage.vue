<script>
import { mapGetters } from 'vuex';
import { PRODUCT_NAME, PAGES } from '../product';

// Rancher management API resource type for clusters
const MGMT_CLUSTER = 'management.cattle.io.cluster';

export default {
  name: 'CattleDriveDashboard',

  async fetch() {
    // Load all downstream clusters from the management store
    this.allClusters = await this.$store.dispatch('management/findAll', { type: MGMT_CLUSTER });
  },

  data() {
    return {
      allClusters:    [],
      sourceCluster:  null,
      targetCluster:  null,
    };
  },

  computed: {
    ...mapGetters({ t: 'i18n/t' }),

    clusterOptions() {
      return (this.allClusters || [])
        // Exclude the local management cluster itself
        .filter(c => c.id !== 'local')
        .map(c => ({
          label: c.spec?.displayName || c.id,
          value: c.id,
        }));
    },

    canContinue() {
      return this.sourceCluster && this.targetCluster && this.sourceCluster !== this.targetCluster;
    },
  },

  methods: {
    goToStatus() {
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.STATUS }`,
        query:  { source: this.sourceCluster, target: this.targetCluster },
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },

    goToMigrate() {
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.MIGRATE }`,
        query:  { source: this.sourceCluster, target: this.targetCluster },
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },
  },
};
</script>

<template>
  <div class="cattle-drive-dashboard">
    <div class="header">
      <h1>
        <i class="icon icon-chevron-right" />
        Cattle Drive
      </h1>
      <p class="subtitle">
        Migrate Rancher objects — Projects, Namespaces, Role Bindings, and Catalog Repos —
        from one downstream cluster to another.
      </p>
    </div>

    <div class="cluster-picker card-container mt-20">
      <div class="card-title">
        Select Clusters
      </div>
      <div class="cluster-picker__row">
        <div class="cluster-picker__field">
          <LabeledSelect
            v-model="sourceCluster"
            label="Source Cluster"
            :options="clusterOptions"
            placeholder="Select source cluster..."
          />
        </div>

        <div class="cluster-picker__arrow">
          <i class="icon icon-chevron-right icon-2x" />
        </div>

        <div class="cluster-picker__field">
          <LabeledSelect
            v-model="targetCluster"
            label="Target Cluster"
            :options="clusterOptions"
            placeholder="Select target cluster..."
          />
        </div>
      </div>

      <div v-if="sourceCluster && targetCluster && sourceCluster === targetCluster" class="mt-10">
        <Banner color="warning" label="Source and target clusters must be different." />
      </div>

      <div class="cluster-picker__actions mt-20">
        <button
          class="btn role-secondary mr-10"
          :disabled="!canContinue"
          @click="goToStatus"
        >
          <i class="icon icon-list-flat mr-5" />
          View Migration Status
        </button>
        <button
          class="btn role-primary"
          :disabled="!canContinue"
          @click="goToMigrate"
        >
          <i class="icon icon-upload mr-5" />
          Run Migration
        </button>
      </div>
    </div>

    <div class="feature-grid mt-30">
      <div class="feature-card">
        <i class="icon icon-folder feature-card__icon" />
        <h3>Projects & Namespaces</h3>
        <p>Migrate custom projects and their namespaces, preserving resource quota configurations.</p>
      </div>
      <div class="feature-card">
        <i class="icon icon-user feature-card__icon" />
        <h3>Role Bindings</h3>
        <p>Migrate ProjectRoleTemplateBindings and ClusterRoleTemplateBindings to maintain user permissions.</p>
      </div>
      <div class="feature-card">
        <i class="icon icon-catalog feature-card__icon" />
        <h3>Catalog Repos</h3>
        <p>Migrate custom Helm catalog repositories so workloads can be re-deployed on the target cluster.</p>
      </div>
      <div class="feature-card">
        <i class="icon icon-diff feature-card__icon" />
        <h3>Drift Detection</h3>
        <p>Identify objects that exist on the target but have diverged in spec from the source.</p>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.cattle-drive-dashboard {
  padding: 20px;

  .header {
    border-bottom: 1px solid var(--border);
    padding-bottom: 15px;

    h1 {
      display: flex;
      align-items: center;
      gap: 10px;
      font-size: 24px;
      font-weight: 600;
    }

    .subtitle {
      margin-top: 8px;
      color: var(--body-text);
      max-width: 700px;
    }
  }

  .card-container {
    background: var(--box-bg);
    border: 1px solid var(--border);
    border-radius: var(--border-radius);
    padding: 20px;

    .card-title {
      font-size: 16px;
      font-weight: 600;
      margin-bottom: 15px;
      color: var(--body-text);
    }
  }

  .cluster-picker {
    &__row {
      display: flex;
      align-items: center;
      gap: 15px;
    }

    &__field {
      flex: 1;
    }

    &__arrow {
      color: var(--primary);
      flex: 0 0 auto;
      padding-top: 8px;
    }

    &__actions {
      display: flex;
      justify-content: flex-end;
    }
  }

  .feature-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 16px;
  }

  .feature-card {
    background: var(--box-bg);
    border: 1px solid var(--border);
    border-radius: var(--border-radius);
    padding: 20px;

    &__icon {
      font-size: 28px;
      color: var(--primary);
      display: block;
      margin-bottom: 10px;
    }

    h3 {
      font-size: 14px;
      font-weight: 600;
      margin-bottom: 6px;
    }

    p {
      font-size: 13px;
      color: var(--body-text);
      line-height: 1.5;
    }
  }
}
</style>
