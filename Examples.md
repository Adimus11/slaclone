# Example messages

## Create room
```json
{
    "action": "create_room",
    "payload": {
        "name": "example_room"
    }
}
```

## Join room
```json
{
    "action": "join_room",
    "payload": {
        "name": "example_room"
    }
}
```

## Leave room
```json
{
    "action": "leave_room",
    "payload": {
        "name": "example_room"
    }
}
```

## Send Message
```json
{
    "action": "send_message",
    "payload": {
        "room": "example_room",
        "message": "Hello, World!"
    }
}
```
