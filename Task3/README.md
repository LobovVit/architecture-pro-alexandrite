# Task3.1 — Go сервисы с OpenTelemetry + Jaeger в Kubernetes (MVP)

---

## Структура

```
  k8s/
    jaeger.yaml
    services.yaml
  services/
    service-a/
      main.go
      go.mod
      Dockerfile
    service-b/
      main.go
      go.mod
      Dockerfile
```

---

## 1) Поднять кластер (пример с minikube)

```bash
minikube start --driver=docker
kubectl get nodes
```

---

## 2) Развернуть Jaeger

```bash
kubectl apply -f Task3/k8s/jaeger.yaml
kubectl -n observability rollout status deploy/simplest
kubectl -n observability get svc
```

---

## 3) Собрать Docker-образы и загрузить в кластер

### Вариант A (minikube): использовать docker-env

```bash
eval $(minikube docker-env)

docker build -t service-a:latest Task3/services/service-a
docker build -t service-b:latest Task3/services/service-b
```

### Вариант B (kind): load image

```bash
docker build -t service-a:latest Task3/services/service-a
docker build -t service-b:latest Task3/services/service-b

kind load docker-image service-a:latest
kind load docker-image service-b:latest
```

---

## 4) Задеплоить сервисы

```bash
kubectl apply -f Task3/k8s/services.yaml
kubectl rollout status deploy/service-b
kubectl rollout status deploy/service-a
kubectl get pods
```

---

## 5) Сгенерировать трейс (service-a вызывает service-b)

```bash
kubectl exec -it $(kubectl get pods -l app=service-a -o jsonpath='{.items[0].metadata.name}') --   wget -qO- http://service-a:8080
```

Ожидаемый ответ:

```
service-a: ok -> called service-b
```

---

## 6) Открыть Jaeger UI

```bash
kubectl -n observability port-forward svc/simplest-query 16686:16686
```

в браузере: http://localhost:16686

### Как найти трейс
- В фильтре Service выбери `service-a`
- **Find Traces**
- Должны быть span’ы:
  - `GET /` (service-a)
  - `GET /` (service-b) как child span

---

## 7) Скриншот

![jaeger-trace](jaeger-trace.png)

---

## Troubleshooting

### Нет трейсов в Jaeger
- проверь, что Jaeger поднят: `kubectl -n observability get pods`
- проверь, что сервисы отправляют OTLP:
  - переменная `OTEL_EXPORTER_OTLP_ENDPOINT` в deployment должна указывать на
    `simplest-collector.observability.svc.cluster.local:4317`
- проверь логи сервисов:
  ```bash
  kubectl logs deploy/service-a
  kubectl logs deploy/service-b
  ```

### DNS / сервисы не резолвятся
- убедись, что `service-b` в namespace `default`
- проверь `kubectl get svc`

---

## Что именно обеспечивает «один трейс»?

- Входящий запрос в `service-a` создаёт root span (otelhttp middleware).
- `service-a` вызывает `service-b` через `otelhttp.Transport`, который:
  - создаёт client span,
  - добавляет `traceparent` header (W3C Trace Context).
- `service-b` принимает заголовок и продолжает trace как child span.
