import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // В старом ucode staleTime стоял у 3 запросов из 415 — при значении
      // по умолчанию 0 каждое монтирование компонента било по бэкенду.
      // Свежесть после изменений даёт invalidateQueries, а не рефетч всего.
      staleTime: 60_000,
      gcTime: 5 * 60_000,
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});
