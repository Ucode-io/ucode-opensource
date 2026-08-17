# 2. Регистрация через Google не реализована

Дата: 2026-08-14
Статус: принято

## Контекст

Экран регистрации по референсу содержит кнопку «Continue with Google».
При разборе контракта выяснилось, что вход и регистрация через Google
на бэкенде устроены по-разному.

**Вход — безопасно.** `session_service_v2.go`, случай `WithGoogle`,
вызывает `helper.GetGoogleUserInfo(token)`, а тот идёт в
`https://www.googleapis.com/oauth2/v3/userinfo?access_token=...`
и получает email от самого Google. Токен проверяет Google.

**Регистрация — небезопасно.** `company_service.go:78` вызывает
`helper.DecodeGoogleIDToken(googleToken)`. Функция (`pkg/helper/email.go:58`)
делает следующее:

```go
parts := strings.Split(idToken, ".")
payload := parts[1]
decoded, _ := base64.URLEncoding.DecodeString(payload)
json.Unmarshal(decoded, &userInfo)
```

Подпись JWT не проверяется. Дальше `userInfo["email"]` и
`userInfo["email_verified"]` принимаются на веру.

Строка вида `x.<base64 от {"email":"чужой@адрес","email_verified":true}>.x`
создаёт компанию на произвольный чужой email, и Google в этом не участвует.
Пароль задаётся в том же запросе — то есть регистрирующий получает
рабочий вход под этим адресом.

Дополнительно: поля называются одинаково (`access_token`), но пути ждут
разные типы токена — вход OAuth access token, регистрация ID token.

## Решение

Вход через Google — реализован. Отправляем OAuth access token,
полученный неявным потоком (`useGoogleLogin` из `@react-oauth/google`),
в `/v3/multicompany/default-login` с `type: "google"`.

Регистрация через Google — **не реализована**. На экране регистрации
только email и пароль. Кнопки Google там нет.

## Последствия

Пользователь, привыкший регистрироваться через Google, проходит на один
шаг больше. Это осознанная цена.

Альтернатива — отправлять неподписанный ID token — означала бы встроить
уязвимость бэкенда в новый фронтенд и сделать её частью рабочего
сценария. Отказ фронтенда пользоваться сломанным путём — единственное,
что здесь можно сделать со стороны клиента.

## Что нужно от бэкенда

Заменить `DecodeGoogleIDToken` на проверку подписи —
`idtoken.Validate(ctx, token, clientID)` из
`google.golang.org/api/idtoken`. Проверять `aud` против нашего
client id и `iss` против `accounts.google.com`.

После этого регистрация через Google возвращается: на фронте это
кнопка и один вызов, форма для неё уже есть.
