import { createApi } from "@ucode/remote-sdk";
import { useEffect, useState } from "react";

/**
 * Страница микрофронтенда. Это обычный компонент в дереве админки:
 * свой корень React заводить не нужно и нельзя.
 */
export default function Page(props) {
  const api = createApi(props);
  const [rows, setRows] = useState([]);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    // Токен, окружение и проект уже внутри оси — своей авторизации нет.
    // Ответ развёрнут: конверт бэкенда снимает админка.
    api.v2
      .get("/items/orders", { params: { limit: 20 } })
      .then((data) => setRows(data?.response ?? []))
      .catch(() => setFailed(true));
    // Ключ активации меняется при каждом заходе — перезапрашиваем.
  }, [props.activationKey]);

  if (failed) return <p>Не удалось загрузить данные</p>;
  return <ul>{rows.map((row) => <li key={row.guid}>{row.name}</li>)}</ul>;
}
