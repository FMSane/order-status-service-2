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
* Transiciones entre estados
  * Pendiente → En Preparación → Enviado → Entregado. (camino feliz)
  * Pendiente → Cancelado. (usuario propietario de la orden)
  * Pendiente → En Preparación → Cancelado. (usuario propietario de la orden)
  * Pendiente → Rechazado. (sólo admin)
  * Pendiente → En Preparación → Rechazado. (sólo admin)

* Otras consideraciones
  * Al establecer el estado de una orden, el sistema comprobará que ese estado no sea el actual de la orden, para así proceder a actualizarlo.
  * Si una orden posee estado "Cancelado", "Rechazado" o "Entregado", ya no se podrá cambiar el estado (estados finales).
  * El sistema establece conexión con Rabbit, recibiendo los mensajes del microservicio Order a través de "order_placed", para inicializar las nuevas órdenes al estado "Pendiente" y asignarles su información de envío, como dirección y comentarios por parte del comprador.
 
## Requisitos previos
* Docker y Docker Compose instalados.
* Microservicio de autenticación (prod-auth-go) en ejecución.
* Microservicio de ordenes (prod-orders-go) en ejecución.
* Rabbit en ejecución, con acceso a "place_order" por parte de Order.
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

### OrderStatus
Representa una orden completa con su estado actual, historial y datos de envío.

``` JSON
OrderStatus {
    "orderId": string,
    "userId": string,
    "status": string,            // estado actual (ej: "Pendiente", "En Preparación", ...)
    "history": StatusRecord[],   // historial completo de cambios de estado
    "shipping": Shipping,        // dirección de entrega
    "createdAt": string (ISO timestamp),
    "updatedAt": string (ISO timestamp)
}
```

### Shipping
Datos asociados al envío de la orden.

``` JSON
Shipping {
    "addressLine1": string,
    "city": string,
    "postalCode": string,
    "province": string,
    "country": string,
    "comments": string
}
```

### StatusRecord
Cada entrada representa un cambio de estado en la orden.
``` JSON
StatusRecord {
    "status": string,             // estado asignado
    "reason": string,             // motivo opcional del cambio
    "userId": string,             // usuario que realizó el cambio
    "timestamp": string (ISO timestamp),
    "current": boolean            // true = este es el último estado
}
```

## Casos de Uso
### 1. Crear estado inicial de una orden (POST /status/init)
Este caso de uso comienza cuando un cliente externo —otro microservicio o una prueba manual— envía una solicitud `POST /status/init` para crear el estado inicial de una orden. Esta operación no requiere token, por lo que el flujo pasa directamente al `OrderController.InitStatus`.

Primero se valida el cuerpo del request usando `c.ShouldBindJSON(&req)`. El objeto recibido debe respetar el DTO `InitOrderStatusRequest`, que exige `orderId` y `userId`. Si algo falta, se devuelve un `400 Bad Request`.

Luego, el controlador llama al servicio mediante:
 `ctl.Service.InitOrderStatus(c.Request.Context(), req.OrderID, req.UserID, req.Shipping, false)`

La lógica continúa dentro de `OrderStatusService.InitOrderStatus`, donde se crea la estructura inicial `OrderStatus`. Si ya existe una orden con un `orderId` igual al recibido por el servicio, no se creará ni modificará nada. Internamente se fuerza el estado “Pendiente” sin importar lo que llegue. También se genera un único registro en el historial (`StatusRecord`) marcado como `Current: true`.
Este método contiene una pequeña lógica: si los datos del shipping está vacío, genera una dirección por defecto. Finalmente, delega en el repositorio para persistir la orden llamando a `repo.Save`.

#### Restricciones importantes
- No requiere autenticación.
- No valida si la orden ya existe: delega ese comportamiento al repositorio, donde puede ser un upsert.
- El estado inicial siempre es Pendiente, nunca otro.
- El historial comienza siempre con un único registro.


#### API `POST /status/init`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Content-Type: application/json`|El cuerpo de la solicitud o respuesta contiene datos en formato JSON|

#### Body:
``` JSON
{
  "orderId": "string",
  "userId": "string",
  "shipping": {
    "addressLine1": "string",
    "city": "string",
    "postalCode": "string",
    "province": "string",
    "country": "string",
    "comments": "string"
    }
}
```

#### Respuesta:
`201`
``` JSON
{
    "orderId": "string",
    "userId": "string",
    "status": "Pendiente",
    "history": [
        {
            "status": "Pendiente",
            "reason": "Orden inicializada",
            "userId": "string",
            "timestamp": "string",
            "current": true
        }
    ],
    "shipping": {
        "addressLine1": "string",
        "city": "string",
        "postalCode": "string",
        "province": "string",
        "country": "string",
        "comments": "string"
    },
    "createdAt": "string",
    "updatedAt": "string"
}
```

`403`
Si el formato no es válido.


### 2. Inicialización mediante RabbitMQ (evento place_order)
Este caso se activa de forma asíncrona cuando llega el evento `order_placed` al exchange fanout.
El archivo `/rabbit/setup.go` crea la cola propia, la bindea y registra un consumidor. Cada mensaje recibido es entregado a `PlaceOrderConsumer.Handle`.

#### Dentro de Handle:
- Se registra el mensaje crudo en logs.
- Se deserializa usando `json.Unmarshal` en un `PlacedOrderMessage`.
- Se construye un `InitOrderStatusRequest`.
- Se llama a:
`Service.InitOrderStatus(context.Background(), event.Message.OrderID, event.Message.UserID, event.Message.Shipping, true)`

La lógica ejecutada es la misma que el caso anterior.

#### Restricciones importantes
- No hay validación de token (los consumidores no usan middleware).
- Si el mensaje es inválido o incompleto, se loguea el error y el mensaje continúa.
- El estado inicial sigue siendo siempre Pendiente.
- El shipping se fuerza a datos predefinidos en el Service si llega vacío.



### 3. Actualizar estado de una orden
Este flujo inicia cuando se realiza un `PATCH hacia /status/{orderId}/status`. Esta operación requiere token, por lo cual pasa primero por `AuthMiddleware`.

El middleware extrae el token del header, llama a `AuthService.ValidateToken`, guarda en el contexto `userID`, `userPermissions` y nombre del usuario.

Una vez dentro del controlador, `UpdateStatus` obtiene el orderId desde la URL, valida el body usando `UpdateStatusRequest` (requiere `status`), recupera datos del contexto como: `actorID`, permisos del usuario, y determina si es administrador usando `slices.Contains`.

Luego llama al servicio:

`Service.UpdateStatus(ctx, orderId, newStatus, reason, actorID, isAdmin)`

#### Lógica central en el servicio
Dentro de `UpdateStatus` ocurren las validaciones más importantes del sistema:
- Buscar la orden. Si no existe → error.
- Verificar estado actual (`current`).
- Si el nuevo estado es igual al actual → no hace nada.
- Si el estado actual es final (Cancelado, Rechazado, Entregado) → bloquea con `ErrFinalState`.
- Validar que el nuevo estado existe (`isValidState`).
- Decidir reglas según rol (`admin` o `user`).

#### Reglas para admin
- No puede poner Cancelado.
- No puede poner Rechazado si la orden ya está en Cancelado, Enviado o Entregado.
- Debe respetar el mapa `adminTransitions`, por ejemplo:
-- Pendiente → En Preparación o Rechazado
-- En Preparación → Enviado o Rechazado
-- Enviado → Entregado
- Si la transición es válida:
-- → crea un nuevo StatusRecord y lo manda a repo.UpdateStatus.

#### Reglas para usuario
- Solo puede operar sobre órdenes donde `ord.UserID` == `actorID`; si no: `ErrForbidden`.
- Puede cancelar mientras la orden no esté en Enviado, Entregado o Rechazado.
- Debe respetar `userTransitions`, como:
-- Pendiente → Cancelado
-- En Preparación → Cancelado

#### Restricciones importantes
- Requiere token válido.
- Los usuarios no pueden cambiar estados de órdenes ajenas (la pertenencia de las órdenes se valida mediante el token).
- Los administradores no pueden cancelar ni rechazar en estados prohibidos.
- Nadie puede modificar una orden ya finalizada.
- Un estado inválido devuelve `ErrInvalidTransition`.
- Control estricto de transiciones: no se puede saltar estados arbitrariamente.


#### API
`PATCH /status/:orderId/status`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin" o "user", dependiendo el caso, en formato JWT|
|`Content-Type: application/json`|El cuerpo de la solicitud o respuesta contiene datos en formato JSON|

#### Body:
``` JSON
{
  "status": "string",
  "reason": "string"
}
```

#### Respuesta:
`201`
``` JSON
{
    "message": "status updated"
}
```

`400`
``` JSON
{
    "error": "forbidden"
}
```
En caso de usuario sin permisos, transición errónea, acción no permitida para ese tipo de usuario.

`400`
``` JSON
{
    "error": "cannot change final state"
}
```
En caso de que la orden se encuentre en un estado final de envío, como Cancelado, Entregado o Rechazado.


### 4. Ver los estados de las órdenes del usuario actual autenticado
Este caso comienza después de pasar por `AuthMiddleware`, que asegura que el usuario esté logueado y deposita `userID` en el contexto (mediante la utilización del token de autenticación, y la conexión con el microservicio Auth).

El controlador `GetMyOrders` toma ese valor y llama a:
`Service.GetByUserID(ctx, userID)`

El servicio delega en el repositorio con `FindByUserID`, que devuelve todas las órdenes asociadas a ese usuario.

#### Restricciones importantes
-Requiere token.
- Solo devuelve órdenes cuyo `userId` coincide con el del token.


### API
`GET /orders/mine`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "user" en formato JWT|

#### Respuesta:
`200`
``` JSON
[
    {
        "orderId": "string",
        "userId": "string",
        "status": "string",
        "history": [
            {
                "status": "string",
                "reason": "string",
                "userId": "string",
                "timestamp": "string",
                "current": false/true
            },
        ],
        "shipping": {
            "addressLine1": "string",
            "city": "string",
            "postalCode": "string",
            "province": "string",
            "country": "string",
            "comments": "string"
        },
        "createdAt": "string",
        "updatedAt": "string"
    },
]
```

`401`
``` JSON
{
    "error": "missing authorization header"
}
```

### 5. Obtener el listado de todas las órdenes y sus estados (sólo admin)
Aquí se utiliza el middleware `AdminOnly`. Antes de llegar al controlador, el middleware revisa `userPermissions`; si el usuario no tiene permisos "admin", devuelve `403 Forbidden`.
Una vez autorizado, el controlador llama a:
`Service.GetAll(ctx)`

El servicio delega en `repo.FindAll`, obteniendo todos los documentos.

#### Restricciones importantes
- Solo administradores pueden acceder.
- No hay filtros adicionales.


#### API
`GET /admin/orders/all`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin" en formato JWT|

#### Respuesta:
`200`
``` JSON
[
    {
        "orderId": "string",
        "userId": "string",
        "status": "string",
        "history": [
            {
                "status": "string",
                "reason": "string",
                "userId": "string",
                "timestamp": "string",
                "current": false/true
            },
        ],
        "shipping": {
            "addressLine1": "string",
            "city": "string",
            "postalCode": "string",
            "province": "string",
            "country": "string",
            "comments": "string"
        },
        "createdAt": "string",
        "updatedAt": "string"
    },
]
```

`403`
``` JSON
{
    "error": "admin privileges required"
}
```


#### 6. Obtener órdenes por estado
El flujo es equivalente al anterior, también protegido por `AdminOnly`.
El controlador toma el parámetro `state`, y llama a:
`Service.GetByStatus(ctx, state)`

Devuelve todas las órdenes cuyo estado actual coincide exactamente con el estado solicitado.

#### Restricciones importantes
- Solo accesible por administradores.
- No se valida que el estado sea uno de los definidos; un estado inexistente simplemente devolverá lista vacía.


#### API
`GET /admin/orders/:state`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin" en formato JWT|

#### Respuesta:
`200`
``` JSON
[
    {
        "orderId": "string",
        "userId": "string",
        "status": "string",
        "history": [
            {
                "status": "string",
                "reason": "string",
                "userId": "string",
                "timestamp": "string",
                "current": false/true
            },
        ],
        "shipping": {
            "addressLine1": "string",
            "city": "string",
            "postalCode": "string",
            "province": "string",
            "country": "string",
            "comments": "string"
        },
        "createdAt": "string",
        "updatedAt": "string"
    },
]
```

`403`
``` JSON
{
    "error": "admin privileges required"
}
```


### 7. Obtener el último estado de una orden

El usuario pasa por `AuthMiddleware`. El controlador recupera `userID` y permisos.
Primero se busca la orden:
`o, err := Service.GetByOrderID(...)`
Si no existe → 404.

Luego ocurre la validación clave:
```
if !isAdmin && o.UserID != actorID {
    return 403
}
```

Es decir:
- Los administradores siempre pueden ver cualquier orden.
- Un usuario común solo puede ver órdenes propias.
- El controlador luego recorre `o.History` para buscar el registro donde `Current == true`, que representa el estado actual.

#### Restricciones importantes
- Usuario debe estar autenticado.
- No se puede consultar información de órdenes ajenas.
- Requiere que la orden tenga un registro marcado como Current (si no, error interno).
- Admin tiene acceso total.


#### API
`GET /orders/:orderId/latest`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin", o ser el usuario propietario de esa orden, en formato JWT|


#### Respuesta:
`200`
``` JSON
{
    "status": "string",
    "reason": "string",
    "userId": "string",
    "timestamp": "string",
    "current": true
}
```

`403`
``` JSON
{
    "error": "you cannot view another user's order"
}
```
En caso de que se intente ver detalles de una orden que no pertenece al usuario autenticado, y que además ese usuario no es "admin".

`403`
``` JSON
{
    "error": "missing authorization header"
}

```
En caso de que el usuario no esté autenticado de ninguna forma.


### 8. Obtener todas las órdenes junto a su último estado
El controlador invoca `Service.GetAll()`, itera cada orden y dentro de cada historial busca el registro Current.
Construye una estructura compacta: `orderId`, `userId`, `status`, `shipping`.

Este endpoint no aplica reglas del negocio: simplemente resume información.

#### Restricciones importantes
- Sólo un usuarios con permisos admin puede acceder a los datos.
- Solo lectura.

#### API
`GET /admin/orders-with-status`

#### Headers
|Cabecera|Contenido|
| --- | --- |
|`Authorization: Bearer xxx`|Token de usuario con permiso "admin" en formato JWT|


#### Respuesta:
`200`
``` JSON
[
  {
        "orderId": "string",
        "shipping": {
            "addressLine1": "string",
            "city": "string",
            "postalCode": "string",
            "province": "string",
            "country": "string",
            "comments": "string"
        },
        "status": "string",
        "userId": "string"
    },
]
```

`403`
``` JSON
{
    "error": "admin privileges required"
}
```
