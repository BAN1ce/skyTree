# Profile

A distributed message broker base on mqtt5.0 protocol.

# Broker Structure

## components

### Listener
A distributed listener for client connect. support TCP and WebSocket protocol.
### Session
A distributed session manager for client implement by Raft.
### Store
Store publish messages. Any database implement store interface. support mysql,redis,leveldb and so on.

```puml
@startuml
cloud "Broker"{
        component "Session" {
        component "SessionStore"
        component "SessionQueue"
        }
        component "Store" {
        component "MessageStore"
        component "MessageQueue"
        }
        component "Listener" {
        component "TCP"
        component "WebSocket"
        }
}
@enduml

```

# Quick Start

# Broker Handler Process

## Connect Handler

```mermaid
sequenceDiagram

Client ->> Broker : request connect

Broker ->> Broker : check protocol & version 

alt set username or password 
Broker ->> Broker : auth username and password
else
end

alt property include auth
Broker ->> Broker : auth with auth data
end

Broker ->> Broker : set keepalive for client


alt flag clean is true 
    Broker ->> Session : clean client's session
    Broker ->> Store : clean store belong client
   
else
    Broker ->> Session : use old session or create new
    Broker ->> Store: try read unack message  or other unfinish task  
end

Broker ->> Session : store property to session

Broker -->> Client : response ack

```

# How Client Read Store With Session

```mermaid
    sequenceDiagram
participant c as Client
participant store as Store
participant session asSession

c ->> store : request topics's message by last message id if exist or emtpy message id
store -->> session : return messages 

alt return zero message
    c ->> session : add once listen client's all topics's publish event
    c -->> c : block wait event trigger
    alt event trigger
        c ->> store: read event's topic's message
        c -->> c : cancel block wait
    end       
end

```
# Store Structure

```mermaid
 classDiagram
    class Packet
      Packet: GetID() string
      Packet: GetPayload() []byte
      Packet: GetOffset() int64
    class Session
    Session: NextPacket() (Packet,bool)
    Session: 
    class Message
      Message: GetOffset() int64
      Message: GetPayload() []byte
    Message --* Queue
    <<Interface>> Queue
    Queue: ReadByOffset(offset int64,limit int) []Message
    Queue: DeleteByOffset(offset int64)
 
```

# API

