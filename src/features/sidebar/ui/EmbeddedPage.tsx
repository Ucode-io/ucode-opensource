/**
 * Страница, встроенная в экран рамкой: пункт меню с адресом
 * (`attributes.website_link`).
 *
 * Так же показывала её и старая админка (`views/Constructor/WebsitePage`)
 * — тем же одним iframe. Разница одна: у нас рамка ограничена в правах.
 * Чужая страница внутри админки видит наш адрес и наше окно; `sandbox`
 * оставляет ей ровно то, без чего сайт не работает, и не даёт ни менять
 * адрес вкладки, ни лезть в наш origin.
 */
export function EmbeddedPage({ url, title }: { url: string; title: string }) {
  return (
    <iframe
      src={url}
      title={title}
      className="min-h-0 flex-1 border-0 bg-surface"
      sandbox="allow-scripts allow-forms allow-popups allow-downloads"
      referrerPolicy="no-referrer"
    />
  );
}
