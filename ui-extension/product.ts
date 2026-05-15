// Rancher Dashboard blank-cluster sentinel value
const BLANK_CLUSTER = '_';

// The product name must be unique across all extensions
export const PRODUCT_NAME = 'cattle-drive';

// Page/route name constants
export const PAGES = {
  DASHBOARD: 'dashboard',
  STATUS:    'status',
  MIGRATE:   'migrate',
};

export function init($plugin: any, store: any) {
  const {
    product,
    virtualType,
    basicType,
  } = $plugin.DSL(store, PRODUCT_NAME);

  // Register a top-level product entry in the sidebar
  product({
    icon:    'chevron-right',
    inStore: 'management',
    weight:  100,
    to:      {
      name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.DASHBOARD }`,
      params: {
        product: PRODUCT_NAME,
        cluster: BLANK_CLUSTER,
      },
    },
  });

  // Dashboard page — cluster picker & overview
  virtualType({
    labelKey: 'cattleDrive.nav.dashboard',
    name:     PAGES.DASHBOARD,
    route:    {
      name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.DASHBOARD }`,
      params: {
        product: PRODUCT_NAME,
        cluster: BLANK_CLUSTER,
      },
    },
  });

  // Status page — migration status for selected clusters
  virtualType({
    labelKey: 'cattleDrive.nav.status',
    name:     PAGES.STATUS,
    route:    {
      name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.STATUS }`,
      params: {
        product: PRODUCT_NAME,
        cluster: BLANK_CLUSTER,
      },
    },
  });

  // Migrate page — run migration with live progress
  virtualType({
    labelKey: 'cattleDrive.nav.migrate',
    name:     PAGES.MIGRATE,
    route:    {
      name:   `${ PRODUCT_NAME }-c-cluster-${ PAGES.MIGRATE }`,
      params: {
        product: PRODUCT_NAME,
        cluster: BLANK_CLUSTER,
      },
    },
  });

  // Register all three pages as side-menu entries
  basicType([PAGES.DASHBOARD, PAGES.STATUS, PAGES.MIGRATE]);
}
