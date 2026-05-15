<script>
import { PRODUCT_NAME, PAGES } from '../product';
import { DEFAULT_PROXY_API_BASE, authHeaders } from '../utils/api';
import { saveNavState } from '../utils/session-store';

export default {
  name: 'CattleDriveDashboard',

  data() {
    return {
      // Prefer Rancher auth-aware in-cluster proxy by default.
      apiBase:       DEFAULT_PROXY_API_BASE,
      kubeconfigPath: '',
      allClusters:   [],
      sourceCluster: null,
      targetCluster: null,
      loadingClusters: false,
      loadError:     null,
    };
  },

  computed: {
    clusterOptions() {
      // Use displayName as the value: the cattle-drive API matches source/target
      // by display name (same convention as the CLI -s / -t flags).
      return (this.allClusters || []).map(c => ({
        label: c.displayName || c.id,
        value: c.displayName,
      }));
    },

    canFetch() {
      return this.apiBase;
    },

    canContinue() {
      return this.sourceCluster &&
        this.targetCluster &&
        this.sourceCluster !== this.targetCluster;
    },

    // RBAC guard: check that the logged-in user can at least read management
    // projects (a prerequisite for any migration operation).  Users without
    // this access should not attempt to proceed.
    canManage() {
      const schema = this.$store?.getters?.['management/schemaFor']?.('management.cattle.io.project');

      return !!schema;
    },
  },

  methods: {
    async fetchClusters() {
      if (!this.canFetch) return;
      this.loadingClusters = true;
      this.loadError = null;
      try {
        const body = {};
        if (this.kubeconfigPath) {
          body.kubeconfig = this.kubeconfigPath;
        }
        const res = await fetch(`${ this.apiBase }/api/clusters`, {
          method:      'POST',
          credentials: 'same-origin',
          headers:     authHeaders(this.$store, this.apiBase),
          body:        JSON.stringify(body),
        });
        const data = await res.json();
        if (!res.ok) {
          throw new Error(data.error || `HTTP ${ res.status }`);
        }
        this.allClusters = data.clusters || [];
        // Reset cluster selections – they may no longer be valid after
        // loading a different kubeconfig or API server.
        this.sourceCluster = null;
        this.targetCluster = null;
      } catch (err) {
        this.loadError = err.message || String(err);
      } finally {
        this.loadingClusters = false;
      }
    },

    // Save navigation state to sessionStorage (keeps sensitive values out of
    // the URL / browser history) before pushing the next route.
    saveState() {
      saveNavState({
        source:     this.sourceCluster,
        target:     this.targetCluster,
        apiBase:    this.apiBase,
        kubeconfig: this.kubeconfigPath,
      });
    },

    goToStatus() {
      this.saveState();
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.STATUS }`,
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },

    goToMigrate() {
      this.saveState();
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.MIGRATE }`,
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
        Migrate Rancher objects between downstream clusters from inside Rancher Dashboard.
      </p>
    </div>

    <!-- RBAC guard: warn users who lack management access -->
    <Banner
      v-if="!canManage"
      color="warning"
      class="mt-20"
      label="You do not have sufficient permissions to perform cluster migrations. Contact your Rancher administrator."
    />

    <!-- Connection settings -->
    <div class="card-container mt-20">
      <div class="card-title">
        Connection Settings
      </div>
      <div class="connection-row">
        <div class="connection-row__field">
          <LabeledInput
            v-model="apiBase"
            label="cattle-drive API Server URL"
            placeholder="http://localhost:8080"
          />
        </div>
        <div class="connection-row__field">
          <LabeledInput
            v-model="kubeconfigPath"
            label="Kubeconfig Path (optional server-side override)"
            placeholder="/path/to/kubeconfig.yaml (optional)"
          />
        </div>
        <div class="connection-row__action">
          <button
            class="btn role-secondary"
            :disabled="!canFetch || loadingClusters"
            @click="fetchClusters"
          >
            <i :class="loadingClusters ? 'icon icon-spinner icon--spin' : 'icon icon-refresh'" />
            Load Clusters
          </button>
        </div>
      </div>
      <Banner v-if="loadError" color="error" :label="loadError" class="mt-10" />
      <Banner
        color="info"
        class="mt-10"
        label="Default mode uses Rancher's authenticated proxy path. Leave kubeconfig empty when the API server uses --default-kubeconfig."
      />
    </div>

    <!-- Cluster Picker -->
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
            :disabled="allClusters.length === 0"
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
            :disabled="allClusters.length === 0"
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
          :disabled="!canContinue || !canManage"
          @click="goToStatus"
        >
          <i class="icon icon-list-flat mr-5" />
          View Migration Status
        </button>
        <button
          class="btn role-primary"
          :disabled="!canContinue || !canManage"
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
        <h3>Projects &amp; Namespaces</h3>
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

  .connection-row {
    display: flex;
    align-items: flex-end;
    gap: 15px;

    &__field { flex: 1; }
    &__action { flex: 0 0 auto; padding-bottom: 2px; }
  }

  .cluster-picker {
    &__row {
      display: flex;
      align-items: center;
      gap: 15px;
    }

    &__field { flex: 1; }

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


<template>
  <div class="cattle-drive-dashboard">
    <div class="header">
      <h1>
        <i class="icon icon-chevron-right" />
        Cattle Drive
      </h1>
      <p class="subtitle">
        Migrate Rancher objects between downstream clusters from inside Rancher Dashboard.
      </p>
    </div>

    <!-- Connection settings -->
    <div class="card-container mt-20">
      <div class="card-title">
        Connection Settings
      </div>
      <div class="connection-row">
        <div class="connection-row__field">
          <LabeledInput
            v-model="apiBase"
            label="cattle-drive API Server URL"
            placeholder="http://localhost:8080"
          />
        </div>
        <div class="connection-row__field">
          <LabeledInput
            v-model="kubeconfigPath"
            label="Kubeconfig Path (optional server-side override)"
            placeholder="/path/to/kubeconfig.yaml (optional)"
          />
        </div>
        <div class="connection-row__action">
          <button
            class="btn role-secondary"
            :disabled="!canFetch || loadingClusters"
            @click="fetchClusters"
          >
            <i :class="loadingClusters ? 'icon icon-spinner icon--spin' : 'icon icon-refresh'" />
            Load Clusters
          </button>
        </div>
      </div>
      <Banner v-if="loadError" color="error" :label="loadError" class="mt-10" />
      <Banner
        color="info"
        class="mt-10"
        label="Default mode uses Rancher's authenticated proxy path. Leave kubeconfig empty when the API server uses --default-kubeconfig."
      />
    </div>

    <!-- Cluster Picker -->
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
            :disabled="allClusters.length === 0"
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
            :disabled="allClusters.length === 0"
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
        <h3>Projects &amp; Namespaces</h3>
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

  .connection-row {
    display: flex;
    align-items: flex-end;
    gap: 15px;

    &__field { flex: 1; }
    &__action { flex: 0 0 auto; padding-bottom: 2px; }
  }

  .cluster-picker {
    &__row {
      display: flex;
      align-items: center;
      gap: 15px;
    }

    &__field { flex: 1; }

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
