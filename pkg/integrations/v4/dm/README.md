# Dimensional Metrics (v4)

The dimensional metrics package (dm) is responsible for processing and emitting the different data types that can be found in a v4 Integration SDK payload.

# Sequence diagram

```mermaid
sequenceDiagram
    Emitter->>+DmEmitter: Send(FwRequest)
    loop FwRequest.Dataset
        alt not IgnoreEntity
            alt not EntityAgent
                DmEmitter->>RegisterEntity (worker): Dataset
            RegisterEntity (worker)->>+DmEmitter: Dataset + EntityID
            end
        end
        DmEmitter->>ExternalPlugin: emitInventory
        DmEmitter->>ExternalPlugin: emitEvent
        DmEmitter->>DmProcessor: metrics
        DmProcessor->>DmEmitter: decorated metrics
        DmEmitter->>MetricsSender: SendMetricsWithCommonAttributes
    end
```
            
