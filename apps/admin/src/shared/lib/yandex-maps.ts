/**
 * Яндекс.Карты v2.1 — загрузка скрипта по требованию.
 *
 * Без npm-обёртки сознательно: карта нужна двум редакторам — точке
 * (MAP) и области (POLYGON), — а обёртка (`@pbe/react-yandex-maps`
 * в старой админке) тянет свой провайдер и контекст ради одного клика
 * по карте. Скрипт грузится при первом открытии редактора и один раз
 * на всё приложение.
 *
 * Ключ берётся из окружения и своего не имеет. Он и так уходит в браузер
 * каждому посетителю, так что секретом не является — но привязан к квоте и
 * биллингу того, кто его выпустил, поэтому в открытом коде ему не место:
 * каждая установка ставит свой. Без ключа редакторы точки и области
 * показывают сообщение вместо карты, остальное приложение работает.
 *
 * Получить ключ: https://developer.tech.yandex.ru/services (JavaScript API).
 */
const API_KEY = import.meta.env.VITE_YANDEX_MAPS_API_KEY ?? "";

/** Ровно то, чем пользуемся. Полных типов у Яндекса нет. */
export type YandexMap = {
  setCenter: (center: [number, number]) => void;
  destroy: () => void;
  geoObjects: { add: (object: YandexPlacemark | YandexPolygon) => void };
  events: {
    add: (name: string, handler: (event: YandexMapEvent) => void) => void;
  };
};

export type YandexPlacemark = {
  geometry: { setCoordinates: (coords: [number, number]) => void };
};

/**
 * Область. Координаты у неё — список КОНТУРОВ, а не точек:
 * `[[[широта, долгота], …]]`. Внешний контур первый, остальные — дырки;
 * мы работаем только с первым (см. features/item, model/polygon).
 *
 * `editor` — надстройка `geoObject.addon.editor`: она приезжает вместе
 * с полным пакетом API, который грузится по умолчанию.
 */
export type YandexPolygon = {
  geometry: {
    getCoordinates: () => number[][][] | null;
    setCoordinates: (rings: number[][][]) => void;
    events: { add: (name: string, handler: () => void) => void };
  };
  editor: {
    /** Ставить точки кликами — для пустой области. */
    startDrawing: () => void;
    /** Двигать уже поставленные — для заполненной. */
    startEditing: () => void;
  };
};

export type YandexMapEvent = { get: (key: "coords") => [number, number] };

export type YandexMapsApi = {
  ready: (callback: () => void) => void;
  Map: new (
    element: HTMLElement,
    state: { center: [number, number]; zoom: number; controls: string[] },
  ) => YandexMap;
  Placemark: new (coords: [number, number]) => YandexPlacemark;
  Polygon: new (
    rings: number[][][],
    properties: Record<string, unknown>,
    options: Record<string, unknown>,
  ) => YandexPolygon;
};

declare global {
  interface Window {
    ymaps?: YandexMapsApi;
  }
}

let pending: Promise<YandexMapsApi> | null = null;

export function loadYandexMaps(locale: string): Promise<YandexMapsApi> {
  if (pending) return pending;

  if (!API_KEY) {
    // Не кэшируем отказ: ключ может появиться к следующему запуску.
    return Promise.reject(
      new Error(
        "Карты не настроены: задайте UCODE_YANDEX_MAPS_API_KEY для этой установки",
      ),
    );
  }

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
