# Order Status Service

Este microservicio se encarga de manejar los estados de las órdenes dentro del sistema, y un catálogo base de estados administrable por usuarios con rol de administrador.
Forma parte del ecosistema de microservicios, y se comunica con el microservicio de autenticación (auth-service) para validar los tokens JWT y los permisos.


## Roles y permisos
* Usuario
  * Ver el estado de sus propias órdenes, junto a los datos de envío.
  * Sólo puede cancelar una orden, si es que esta no está en estado "Enviado", "Entregado" ni "Rechazado". 
* Administrador
  * Cambiar el estado de cualquier orden, excepto al estado "Cancelado".
  * Ver los estados e historial de todas las órdenes, junto a los datos de envío.
  * Puede "Rechazar" una orden, si es que esta no está en estado "Cancelado", "Enviado" ni "Entregado".

* Otras consideraciones
  * Al establecer el estado de una orden, el sistema comprobará que ese estado no sea el actual de la orden, para así proceder a actualizarlo.
  * Si una orden posee estado "Cancelado", "Rechazado" o "Entregado", ya no se podrá cambiar el estado (estados finales).
  * El sistema establece conexión con Rabbit, recibiendo los mensajes del microservicio Order a través de "order_placed", para inicializar las nuevas órdenes al estado "Pendiente" y asignarles su información de envío, como dirección y comentarios por parte del comprador.
 
## Requisitos previos
* Docker y Docker Compose instalados.
* Microservicio de autenticación (prod-auth-go) en ejecución.
* Microservicio de ordenes (prod-orders-go) en ejecución.
* Base de datos MongoDB accesible (puede estar en otro contenedor).


## Build y ejecución con Docker
En la raíz del proyecto abrir una nueva terminal y ejecutar:
``` bash
docker-compose up --build
```


## Autenticación
Cada endpoint que modifica información requiere un token JWT válido.
El token se valida comunicándose con el microservicio de autenticación configurado en AUTH_SERVICE_URL.
Una vez obtenido el token correspondiente según el caso de uso (admin o user), agregar en Postman o el cliente HTTP:
``` bash
Authorization: Bearer <TOKEN>
```


## Modelo de Datos

### Catálogo de estados

``` JSON
{
  "id": string,
  "name": string,
  "created_at": string
}
```

Estado de orden

``` JSON
{
    "id": string,
    "order_id": string,
    "user_id": string,
    "status_id": string,
    "status": string,
    "shipping": {
        "address_line1": string,
        "city": string,
        "province": string,
        "country": string,
        "comments": string
    },
    "created_at": string,
    "updated_at": string
}
```

## API

### 1. Estados base del catálogo (solo administradores)

Estados válidos que pueden asignarse a las órdenes.

### Ver todos los estados del catálogo
`
GET /admin/status/catalog
`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permisos "admin" en formato JWT|


#### Respuesta:
`200`
``` JSON
[
  { "name": "Pendiente" },
  { "name": "En preparación" },
  { "name": "Enviado" },
  { "name": "Entregado" },
  { "name": "Cancelado" }
]


`403`
``` JSON
{
    "error": "admin privileges required"
}
```


#### Crear nuevo estado en el catálogo
`POST /admin/status/catalog`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permisos "admin" en formato JWT|
|`Content-Type: application/json`|El cuerpo de la solicitud o respuesta contiene datos en formato JSON|

#### Body:
``` JSON
{
  "name": "string"
}
```

#### Respuesta:
`200`
``` JSON
{
  "message": "status added to catalog"
}
```

`403`
``` JSON
{
    "error": "admin privileges required"
}
```

### 2. Estados de órdenes reales

#### (Automático) Crear un nuevo estado al realizar una orden
Cuando el microservicio de órdenes registra una nueva orden, debe hacer un POST al siguiente endpoint para inicializar su estado en “Pendiente”:
`
POST /status/init
`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Content-Type: application/json`|El cuerpo de la solicitud o respuesta contiene datos en formato JSON|

#### Body:
``` JSON
{
    "order_id": "string",
    "user_id": "string",
    "shipping": {
        "address_line1": "string",
        "city": "string",
        "province": "string",
        "country": "string",
        "postal_code": "number",
        "comments": "string"
    }
}
```

#### Respuesta:
`201`
``` JSON
{
    "id": "string",
    "order_id": "string",
    "user_id": "string",
    "status_id": "string",
    "status": "Pendiente",
    "shipping": {
        "address_line1": "string",
        "city": "string",
        "province": "string",
        "country": "string",
        "postal_code": "number",
        "comments": "string"
    },
    "created_at": "0001-01-01T00:00:00Z",
    "updated_at": "2025-11-17T22:01:43.2253701Z"
}
```

`403`
Si el formato no es válido.

#### Cambiar el estado de una orden (solo admin)
`PUT /status/:object_status_order_id`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin" o "user", dependiendo el caso, en formato JWT|
|`Content-Type: application/json`|El cuerpo de la solicitud o respuesta contiene datos en formato JSON|

#### Body:
``` JSON
{
  "status_id": "string"
}
```

#### Respuesta:
`201`
``` JSON
{
    "id": "string",
    "order_id": "string",
    "user_id": "string",
    "status_id": "string",
    "status": "string",
    "shipping": {
        "address_line1": "string",
        "city": "string",
        "country": "string"
    },
    "created_at": "0001-01-01T00:00:00Z",
    "updated_at": "2025-11-17T22:09:32.012Z"
}
```

`500`
``` JSON
{
    "error": "only admin or seller can reject the order"
}
```

`500`
``` JSON
{
    "error": "only client can cancel the order"
}
```

`500`
``` JSON
{
    "error": "cannot change status from terminal state 'Entregado'"
}
```

`500`
``` JSON
{
    "error": "cannot change status from terminal state 'Cancelado'"
}
```

`500`
``` JSON
{
    "error": "cannot change status from terminal state 'Rechazado'"
}
```

#### Ver los estados de las órdenes del usuario actual autenticado
`GET /status`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "user" en formato JWT|

#### Respuesta:
`200`
``` JSON
[
    {
        "id": "string",
        "order_id": "string",
        "user_id": "string",
        "status_id": "string",
        "status": "string",
        "shipping": {
            "address_line1": "string",
            "city": "string",
            "country": "string"
        },
        "created_at": "0001-01-01T00:00:00Z",
        "updated_at": "2025-11-15T03:23:59.148Z"
    }
]
```

`401`
``` JSON
{
    "error": "missing authorization header"
}
```

#### Obtener el listado de todas las órdenes y sus estados (sólo admin)
`GET status/all`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin" en formato JWT|

#### Respuesta:
`200`
``` JSON
[
    {
        "id": "string",
        "order_id": "string",
        "user_id": "string",
        "status_id": "string",
        "status": "string",
        "shipping": {
            "address_line1": "string",
            "city": "string",
            "country": "string"
        },
        "created_at": "0001-01-01T00:00:00Z",
        "updated_at": "2025-11-15T03:23:59.148Z"
    }
]
```

`403`
``` JSON
{
    "error": "forbidden: admin access required"
}
```


#### Obtener ordenes por estado
`GET /status/filter?status_id=:status_id`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin" en formato JWT|

#### Respuesta:
`200`
``` JSON
[
    {
        "id": "string",
        "order_id": "string",
        "user_id": "string",
        "status_id": "string",
        "status": "string",
        "shipping": {
            "address_line1": "string",
            "city": "string",
            "country": "string"
        },
        "created_at": "0001-01-01T00:00:00Z",
        "updated_at": "2025-11-15T03:23:59.148Z"
    }
]
```

`403`
``` JSON
{
    "error": "forbidden: admin access required"
}
```
