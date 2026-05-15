<script>
import { PRODUCT_NAME, PAGES } from '../product';

const STEP = {
  IDLE:    'idle',
  RUNNING: 'running',
  SUCCESS: 'success',
  ERROR:   'error',
};

export default {
  name: 'CattleDriveMigrate',

  async fetch() {
    const q = this.$route.query;
    this.sourceId       = q.source     || '';
    this.targetId       = q.target     || '';
    this.apiBase        = q.apiBase    || 'http://localhost:8080';
    this.kubeconfigPath = q.kubeconfig || '';
  },

  data() {
    return {
      sourceId:       '',
      targetId:       '',
      apiBase:        'http://localhost:8080',
      kubeconfigPath: '',
      migrationLog:   [],
      overallStatus:  STEP.IDLE,
      errorMsg:       null,
    };
  },

  computed: {
    isRunning() {
      return this.overallStatus === STEP.RUNNING;
    },

    isIdle() {
      return this.overallStatus === STEP.IDLE;
    },

    isDone() {
      return this.overallStatus === STEP.SUCCESS || this.overallStatus === STEP.ERROR;
    },

    successCount() {
      return this.migrationLog.filter(l => !l.error).length;
    },

    errorCount() {
      return this.migrationLog.filter(l => l.error).length;
    },
  },

  methods: {
    goToDashboard() {
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.DASHBOARD }`,
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },

    goToStatus() {
      this.$router.push({
        name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.STATUS }`,
        query:  {
          source:     this.sourceId,
          target:     this.targetId,
          apiBase:    this.apiBase,
          kubeconfig: this.kubeconfigPath,
        },
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },

    async runMigration() {
      this.migrationLog  = [];
      this.overallStatus = STEP.RUNNING;
      this.errorMsg      = null;

      try {
        const res = await fetch(`${ this.apiBase }/api/migrate`, {
          method:  'POST',
          headers: { 'Content-Type': 'application/json' },
          body:    JSON.stringify({
            kubeconfig: this.kubeconfigPath,
            source:     this.sourceId,
            target:     this.targetId,
          }),
        });

        const data = await res.json();
        if (!res.ok) {
          throw new Error(data.error || `HTTP ${ res.status }`);
        }

        this.migrationLog  = data.log || [];
        this.overallStatus = data.success ? STEP.SUCCESS : STEP.ERROR;
        if (!data.success) {
          this.errorMsg = data.error || 'Migration failed.';
        }
      } catch (err) {
        this.errorMsg      = err.message || String(err);
        this.overallStatus = STEP.ERROR;
      }

      // Scroll log to bottom once results are rendered.
      this.$nextTick(() => {
        const body = this.$el.querySelector('.migration-log__body');
        if (body) {
          body.scrollTop = body.scrollHeight;
        }
      });
    },

    stepIcon(entry) {
      if (entry.error) return 'icon-close text-error';
      return 'icon-checkmark text-success';
    },
  },
};
</script>

<template>
  <div class="cattle-drive-migrate">
    <!-- Header -->
    <div class="page-header">
      <div class="page-header__left">
        <button class="btn btn-sm role-link" @click="goToDashboard">
          <i class="icon icon-chevron-left" /> Back
        </button>
        <h1>Run Migration</h1>
      </div>
    </div>

    <!-- Cluster strip -->
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

    <!-- No clusters warning -->
    <Banner
      v-if="!sourceId || !targetId || !kubeconfigPath"
      color="warning"
      class="mt-20"
      label="No clusters selected. Please go back and configure the connection settings."
    />

    <template v-else>
      <!-- Pre-flight info -->
      <div v-if="isIdle" class="preflight mt-20">
        <Banner color="info" class="mb-15">
          <template #default>
            <p>
              This will migrate all non-default objects (Projects, Namespaces, PRTBs, CRTBs,
              Catalog Repos) from <strong>{{ sourceId }}</strong> to
              <strong>{{ targetId }}</strong>. Objects that already exist on the target
              cluster will be skipped.
            </p>
          </template>
        </Banner>

        <div class="text-right">
          <button class="btn role-secondary mr-10" @click="goToStatus">
            <i class="icon icon-list-flat" /> View Status First
          </button>
          <button class="btn role-primary" @click="runMigration">
            <i class="icon icon-upload" /> Start Migration
          </button>
        </div>
      </div>

      <!-- Running spinner -->
      <div v-if="isRunning" class="running-indicator mt-20">
        <i class="icon icon-spinner icon--spin mr-10" />
        Migration in progress…
      </div>

      <!-- Migration log (shown after run) -->
      <div v-if="migrationLog.length > 0" class="migration-log mt-20">
        <div class="migration-log__header">
          <span>Migration Log</span>
          <span v-if="isDone" class="migration-log__summary">
            <span class="text-success">{{ successCount }} succeeded</span>
            <span v-if="errorCount" class="text-error ml-10">{{ errorCount }} failed</span>
          </span>
        </div>

        <div class="migration-log__body">
          <div
            v-for="(entry, idx) in migrationLog"
            :key="idx"
            class="log-entry"
            :class="entry.error ? 'log-entry--error' : ''"
          >
            <i :class="`icon ${ stepIcon(entry) } log-entry__icon`" />
            <span class="log-entry__label">{{ entry.message }}</span>
          </div>
        </div>
      </div>

      <!-- Overall result -->
      <div v-if="isDone" class="result mt-20">
        <Banner
          v-if="overallStatus === 'success'"
          color="success"
          :label="`Migration complete. ${ successCount } log entries${ errorCount ? ', ' + errorCount + ' errors.' : '.' }`"
        />
        <Banner
          v-else-if="overallStatus === 'error'"
          color="error"
          :label="errorMsg || 'Migration encountered an unexpected error.'"
        />

        <div class="result__actions mt-15">
          <button class="btn role-secondary mr-10" @click="goToStatus">
            <i class="icon icon-list-flat" /> View Status
          </button>
          <button class="btn role-primary" @click="runMigration">
            <i class="icon icon-refresh" /> Run Again
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<style lang="scss" scoped>
.cattle-drive-migrate {
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

    &__arrow { color: var(--primary); }
  }

  .preflight {
    max-width: 680px;
  }

  .running-indicator {
    display: flex;
    align-items: center;
    font-size: 14px;
    color: var(--body-text);
  }

  .migration-log {
    border: 1px solid var(--border);
    border-radius: var(--border-radius);
    overflow: hidden;
    max-height: 500px;
    display: flex;
    flex-direction: column;

    &__header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 10px 16px;
      background: var(--box-bg);
      font-weight: 600;
      font-size: 14px;
      border-bottom: 1px solid var(--border);
      flex: 0 0 auto;
    }

    &__body {
      overflow-y: auto;
      flex: 1;
    }

    &__summary {
      font-weight: 400;
      font-size: 13px;
    }
  }

  .log-entry {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
    font-size: 13px;

    &:last-child { border-bottom: none; }

    &--error { background: var(--error-banner-bg); }

    &__icon  { font-size: 14px; flex: 0 0 auto; margin-top: 2px; }
    &__label { flex: 1; white-space: pre-wrap; word-break: break-word; }
  }

  .result {
    max-width: 680px;

    &__actions { display: flex; }
  }
}
</style>
