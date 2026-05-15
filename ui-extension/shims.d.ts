declare module '*.vue' {
  const component: any;
  export default component;
}

declare module '@shell/core/types' {
  export interface IPlugin {
    metadata?: any;
    addProduct(product: any): void;
    addRoutes(routes: any): void;
  }
}

declare function require(path: string): any;
