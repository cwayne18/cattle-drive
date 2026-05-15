import DashboardPage from '../pages/DashboardPage.vue';
import StatusPage from '../pages/StatusPage.vue';
import MigratePage from '../pages/MigratePage.vue';

import { PRODUCT_NAME, PAGES } from '../product';

const BLANK_CLUSTER = '_';

const routes = [
  {
    name:      `${ PRODUCT_NAME }-c-cluster-${ PAGES.DASHBOARD }`,
    path:      `/${ PRODUCT_NAME }/c/:cluster/${ PAGES.DASHBOARD }`,
    component: DashboardPage,
    meta:      {
      product: PRODUCT_NAME,
      cluster: BLANK_CLUSTER,
    },
  },
  {
    name:      `${ PRODUCT_NAME }-c-cluster-${ PAGES.STATUS }`,
    path:      `/${ PRODUCT_NAME }/c/:cluster/${ PAGES.STATUS }`,
    component: StatusPage,
    meta:      {
      product: PRODUCT_NAME,
      cluster: BLANK_CLUSTER,
    },
  },
  {
    name:      `${ PRODUCT_NAME }-c-cluster-${ PAGES.MIGRATE }`,
    path:      `/${ PRODUCT_NAME }/c/:cluster/${ PAGES.MIGRATE }`,
    component: MigratePage,
    meta:      {
      product: PRODUCT_NAME,
      cluster: BLANK_CLUSTER,
    },
  },
];

export default routes;
