# Telemetry Sampling

Glassbox samples high-frequency trace events before exporting them to the telemetry backend. Sampling reduces export volume while preserving meaningful diagnostic signal.

## How it works

A `Sampler` is applied to trace-related telemetry events. Each event is independently accepted or dropped based on a configured probability. At the default rate of `1.0` every event is emitted; lower values reduce volume proportionally.

## Configuration

### Environment variable

```bash
export GLASSBOX_TELEMETRY_SAMPLE_RATE=0.1   # emit ~10% of trace events
```

### Config file (`~/.Glassbox/config.json`)

```json
{
  "telemetry_sample_rate": 0.1
}
```

### Valid values

| Value | Behaviour |
|-------|-----------|
| `1.0` | All events emitted (default) |
| `0.5` | ~50% of events emitted |
| `0.1` | ~10% of events emitted |
| `0.0` | No trace events emitted |

Values outside `[0.0, 1.0]` are clamped to the nearest bound.

## Defaults

- Default sample rate: `1.0` (all events)
- Telemetry is **opt-in** and disabled by default (`telemetry_enabled: false`)

## Related settings

| Env var | Config key | Description |
|---------|------------|-------------|
| `GLASSBOX_TELEMETRY` | `telemetry_enabled` | Enable/disable telemetry |
| `GLASSBOX_TELEMETRY_ENDPOINT` | `telemetry_endpoint` | OTLP exporter URL |
| `GLASSBOX_TELEMETRY_SAMPLE_RATE` | `telemetry_sample_rate` | Trace event sample rate |

Run `glassbox telemetry` to inspect the current telemetry state.
