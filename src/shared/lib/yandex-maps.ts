/**
 * Яндекс.Карты v2.1 — загрузка скрипта по требованию.
 *
 * Без npm-обёртки сознательно: карта нужна одному редактору поля MAP,
 * а обёртка (`@pbe/react-yandex-maps` в старой админке) тянет свой
 * провайдер и контекст ради одного клика по карте. Скрипт грузится
 * при первом открытии редактора и один раз на всё приложение.
 *
 * Ключ — тот же, что зашит в старой админке: это ключ проекта ucode,
 * и он уже публичный, поскольку уходит в браузер каждому пользователю.
 */
const API_KEY = "5e5a73bd-6e0a-40f1-ba8e-f0b98d95e75f";

/** Ровно то, чем пользуемся. Полных типов у Яндекса нет. */
export type YandexMap = {
  setCenter: (center: [number, number]) => void;
  destroy: () => void;
  geoObjects: { add: (mark: YandexPlacemark) => void };
  events: {
    add: (name: string, handler: (event: YandexMapEvent) => void) => void;
  };
};

export type YandexPlacemark = {
  geometry: { setCoordinates: (coords: [number, number]) => void };
};

export type YandexMapEvent = { get: (key: "coords") => [number, number] };

export type YandexMapsApi = {
  ready: (callback: () => void) => void;
  Map: new (
    element: HTMLElement,
    state: { center: [number, number]; zoom: number; controls: string[] },
  ) => YandexMap;
  Placemark: new (coords: [number, number]) => YandexPlacemark;
};

declare global {
  interface Window {
    ymaps?: YandexMapsApi;
  }
}

let pending: Promise<YandexMapsApi> | null = null;

export function loadYandexMaps(locale: string): Promise<YandexMapsApi> {
  if (pending) return pending;

  pending = new Promise<YandexMapsApi>((resolve, reject) => {
    // Узбекского у Яндекса нет — остальным отдаём английский.
    const lang = locale.startsWith("ru") ? "ru_RU" : "en_US";
    const script = document.createElement("script");

    script.src = `https://api-maps.yandex.ru/2.1/?apikey=${API_KEY}&lang=${lang}`;
    script.async = true;
    script.onload = () => {
      const api = window.ymaps;
      if (!api) return reject(new Error("ymaps не загрузился"));
      api.ready(() => resolve(api));
    };
    script.onerror = () => {
      // Следующая попытка загрузит скрипт заново, а не упрётся в отказ.
      pending = null;
      reject(new Error("Яндекс.Карты недоступны"));
    };

    document.head.append(script);
  });

  return pending;
}
