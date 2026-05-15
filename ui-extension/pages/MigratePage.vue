<script>
import { mapGetters } from 'vuex';
import { PRODUCT_NAME, PAGES } from '../product';

const MGMT_CLUSTER = 'management.cattle.io.cluster';
const MGMT_PROJECT = 'management.cattle.io.project';
const MGMT_PRTB    = 'management.cattle.io.projectroletemplatebinding';
const MGMT_CRTB    = 'management.cattle.io.clusterroletemplatebinding';
const CATALOG_REPO = 'catalog.cattle.io.clusterrepo';

const SKIP_PROJECTS    = ['Default', 'System'];
const SKIP_CRTB_NAMES  = ['creator-cluster-owner'];
const SKIP_CRTB_PREFIX = ['fleet-default-owner'];
const SKIP_REPO_PREFIX = 'rancher-';

const STEP = {
  IDLE:       'idle',
  RUNNING:    'running',
  SUCCESS:    'success',
  ERROR:      'error',
};

export default {
  name: 'CattleDriveMigrate',

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
  },

  data() {
    return {
      sourceId:      '',
      targetId:      '',
      sourceCluster: null,
      targetCluster: null,
      migrationLog:  [],
      overallStatus: STEP.IDLE,
      errorMsg:      null,
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

    isRunning() {
      return this.overallStatus === STEP.RUNNING;
    },

    isDone() {
      return this.overallStatus === STEP.SUCCESS || this.overallStatus === STEP.ERROR;
    },

    successCount() {
      return this.migrationLog.filter(l => l.status === STEP.SUCCESS).length;
    },

    errorCount() {
      return this.migrationLog.filter(l => l.status === STEP.ERROR).length;
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
        query:  { source: this.sourceId, target: this.targetId },
        params: { product: PRODUCT_NAME, cluster: '_' },
      });
    },

    addLog(label, status, detail = '') {
      this.migrationLog.push({
        id:     this.migrationLog.length,
        label,
        status,
        detail,
        ts:     new Date().toLocaleTimeString(),
      });
    },

    updateLog(id, status, detail = '') {
      const entry = this.migrationLog.find(l => l.id === id);
      if (entry) {
        entry.status = status;
        entry.detail = detail;
      }
    },

    async runMigration() {
      this.migrationLog  = [];
      this.overallStatus = STEP.RUNNING;
      this.errorMsg      = null;

      try {
        // ── 1. Load source objects ───────────────────────────────────────
        this.addLog('Loading source cluster objects…', STEP.RUNNING);
        const [allProjects, allPRTBs, allCRTBs, allRepos] = await Promise.all([
          this.$store.dispatch('management/findAll', { type: MGMT_PROJECT }),
          this.$store.dispatch('management/findAll', { type: MGMT_PRTB }),
          this.$store.dispatch('management/findAll', { type: MGMT_CRTB }),
          this.$store.dispatch('management/findAll', { type: CATALOG_REPO }),
        ]);
        this.updateLog(0, STEP.SUCCESS);

        const srcProjects = allProjects.filter(
          p => p.spec?.clusterName === this.sourceId && !SKIP_PROJECTS.includes(p.spec?.displayName)
        );
        const tgtProjects = allProjects.filter(
          p => p.spec?.clusterName === this.targetId
        );
        const srcCRTBs = allCRTBs
          .filter(c => c.metadata?.namespace === this.sourceId)
          .filter(c => !SKIP_CRTB_NAMES.includes(c.name) && !SKIP_CRTB_PREFIX.some(p => c.name?.startsWith(p)));

        const srcRepos = allRepos.filter(
          r => !r.metadata?.name?.startsWith(SKIP_REPO_PREFIX)
        );

        // ── 2. Migrate Projects ──────────────────────────────────────────
        for (const sp of srcProjects) {
          const displayName = sp.spec?.displayName;
          const exists      = tgtProjects.find(p => p.spec?.displayName === displayName);

          if (exists) {
            this.addLog(`Project "${ displayName }"`, STEP.SUCCESS, 'Already migrated — skipped.');
            continue;
          }

          const logId = this.migrationLog.length;
          this.addLog(`Migrating project "${ displayName }"…`, STEP.RUNNING);

          try {
            // Build a minimal new project payload for the target cluster
            const newProject = {
              type:       MGMT_PROJECT,
              apiVersion: 'management.cattle.io/v3',
              kind:       'Project',
              metadata:   {
                namespace:   this.targetId,
                annotations: this.stripLifecycleAnnotations(sp.metadata?.annotations || {}),
              },
              spec: {
                ...sp.spec,
                clusterName: this.targetId,
              },
            };

            await this.$store.dispatch('management/create', newProject);
            this.updateLog(logId, STEP.SUCCESS, 'Done.');
          } catch (err) {
            this.updateLog(logId, STEP.ERROR, err?.message || String(err));
          }

          // Migrate PRTBs for this project
          const srcPRTBs = allPRTBs
            .filter(b => b.projectName === sp.id || b.spec?.projectName === sp.id)
            .filter(b => !['creator-project-owner', 'creator-project-member'].includes(b.name));

          for (const pb of srcPRTBs) {
            const pbLogId = this.migrationLog.length;
            this.addLog(`  Migrating PRTB "${ pb.name }"…`, STEP.RUNNING);
            try {
              const newPRTB = {
                type:       MGMT_PRTB,
                apiVersion: 'management.cattle.io/v3',
                kind:       'ProjectRoleTemplateBinding',
                metadata:   {
                  name:      pb.name,
                  namespace: this.targetId,
                },
                roleTemplateName: pb.roleTemplateName,
                userName:         pb.userName,
                groupName:        pb.groupName,
                groupPrincipalName: pb.groupPrincipalName,
                userPrincipalName:  pb.userPrincipalName,
              };
              await this.$store.dispatch('management/create', newPRTB);
              this.updateLog(pbLogId, STEP.SUCCESS, 'Done.');
            } catch (err) {
              this.updateLog(pbLogId, STEP.ERROR, err?.message || String(err));
            }
          }
        }

        // ── 3. Migrate CRTBs ────────────────────────────────────────────
        for (const sc of srcCRTBs) {
          const logId = this.migrationLog.length;
          this.addLog(`Migrating CRTB "${ sc.name }"…`, STEP.RUNNING);
          try {
            const newCRTB = {
              type:       MGMT_CRTB,
              apiVersion: 'management.cattle.io/v3',
              kind:       'ClusterRoleTemplateBinding',
              metadata:   {
                name:      sc.name,
                namespace: this.targetId,
              },
              clusterName:        this.targetId,
              roleTemplateName:   sc.roleTemplateName,
              userName:           sc.userName,
              groupName:          sc.groupName,
              groupPrincipalName: sc.groupPrincipalName,
              userPrincipalName:  sc.userPrincipalName,
            };
            await this.$store.dispatch('management/create', newCRTB);
            this.updateLog(logId, STEP.SUCCESS, 'Done.');
          } catch (err) {
            this.updateLog(logId, STEP.ERROR, err?.message || String(err));
          }
        }

        // ── 4. Migrate Catalog Repos ─────────────────────────────────────
        for (const sr of srcRepos) {
          const name = sr.metadata?.name;
          const logId = this.migrationLog.length;
          this.addLog(`Migrating catalog repo "${ name }"…`, STEP.RUNNING);
          try {
            const newRepo = {
              type:       CATALOG_REPO,
              apiVersion: 'catalog.cattle.io/v1',
              kind:       'ClusterRepo',
              metadata:   { name },
              spec:       { ...sr.spec },
            };
            await this.$store.dispatch('management/create', newRepo);
            this.updateLog(logId, STEP.SUCCESS, 'Done.');
          } catch (err) {
            this.updateLog(logId, STEP.ERROR, err?.message || String(err));
          }
        }

        this.overallStatus = STEP.SUCCESS;
      } catch (err) {
        this.errorMsg      = err?.message || String(err);
        this.overallStatus = STEP.ERROR;
      }
    },

    stripLifecycleAnnotations(annotations) {
      const out = {};
      for (const [k, v] of Object.entries(annotations)) {
        if (!k.includes('lifecycle.cattle.io')) {
          out[k] = v;
        }
      }
      return out;
    },

    stepIcon(status) {
      switch (status) {
      case STEP.RUNNING: return 'icon-spinner icon--spin';
      case STEP.SUCCESS: return 'icon-checkmark text-success';
      case STEP.ERROR:   return 'icon-close text-error';
      default:           return 'icon-dot';
      }
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
        <strong>Source:</strong> {{ sourceName }}
      </span>
      <i class="icon icon-chevron-right cluster-strip__arrow" />
      <span class="cluster-strip__item">
        <i class="icon icon-server" />
        <strong>Target:</strong> {{ targetName }}
      </span>
    </div>

    <!-- No clusters warning -->
    <Banner
      v-if="!sourceId || !targetId"
      color="warning"
      class="mt-20"
      label="No clusters selected. Please go back and select source and target clusters."
    />

    <template v-else>
      <!-- Pre-flight info -->
      <div v-if="overallStatus === 'idle'" class="preflight mt-20">
        <Banner color="info" class="mb-15">
          <template #default>
            <p>
              This will migrate all non-default objects (Projects, PRTBs, CRTBs, Catalog Repos)
              from <strong>{{ sourceName }}</strong> to <strong>{{ targetName }}</strong>.
              Objects that already exist on the target cluster will be skipped.
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

      <!-- Migration log -->
      <div v-if="migrationLog.length > 0" class="migration-log mt-20">
        <div class="migration-log__header">
          <span>Migration Progress</span>
          <span v-if="isDone" class="migration-log__summary">
            <span class="text-success">{{ successCount }} succeeded</span>
            <span v-if="errorCount" class="text-error ml-10">{{ errorCount }} failed</span>
          </span>
        </div>

        <div class="migration-log__body">
          <div
            v-for="entry in migrationLog"
            :key="entry.id"
            class="log-entry"
            :class="`log-entry--${ entry.status }`"
          >
            <i :class="`icon ${ stepIcon(entry.status) } log-entry__icon`" />
            <span class="log-entry__label">{{ entry.label }}</span>
            <span v-if="entry.detail" class="log-entry__detail">{{ entry.detail }}</span>
            <span class="log-entry__time">{{ entry.ts }}</span>
          </div>
        </div>
      </div>

      <!-- Overall result -->
      <div v-if="isDone" class="result mt-20">
        <Banner
          v-if="overallStatus === 'success'"
          color="success"
          :label="`Migration complete. ${ successCount } objects processed successfully${ errorCount ? ', ' + errorCount + ' failed.' : '.' }`"
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
    display: grid;
    grid-template-columns: 20px 1fr auto auto;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
    font-size: 13px;

    &:last-child { border-bottom: none; }

    &--error  { background: var(--error-banner-bg); }
    &--success { /* default */ }

    &__icon  { font-size: 14px; }
    &__label { }
    &__detail {
      color: var(--body-text);
      font-style: italic;
      font-size: 12px;
    }
    &__time {
      color: var(--body-text);
      font-size: 11px;
      white-space: nowrap;
    }
  }

  .result {
    max-width: 680px;

    &__actions { display: flex; }
  }
}
</style>
