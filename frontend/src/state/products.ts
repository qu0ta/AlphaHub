import { productsApi } from "../api/client";
import { useResource } from "../hooks/useResource";

/** Каталог кредитных продуктов. */
export function useProducts() {
  const { data, loading, error, reload } = useResource(() => productsApi.list(), []);
  return { products: data ?? [], loading, error, retry: reload };
}
