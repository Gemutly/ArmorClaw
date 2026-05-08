package voice

import (
	"context"
	"encoding/binary"
	"math"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
)

type VADEventType int

const (
	VADEventSilence     VADEventType = iota
	VADEventSpeechStart
	VADEventSpeechEnd
)

func (t VADEventType) String() string {
	switch t {
	case VADEventSpeechStart:
		return "speech_start"
	case VADEventSpeechEnd:
		return "speech_end"
	case VADEventSilence:
		return "silence"
	default:
		return "unknown"
	}
}

type VADEvent struct {
	Type      VADEventType
	Timestamp time.Time
	Energy    float64
}

type VADState int

const (
	VADStateSilence VADState = iota
	VADStateSpeech
)

type EnergyVADConfig struct {
	EnergyThreshold    float64
	FrameDurationMs    int
	SilenceDurationMs  int
	SampleRate         uint32
}

func DefaultEnergyVADConfig() EnergyVADConfig {
	return EnergyVADConfig{
		EnergyThreshold:   0.01,
		FrameDurationMs:   20,
		SilenceDurationMs: 300,
		SampleRate:        16000,
	}
}

func (c EnergyVADConfig) FrameSize() int {
	return int(uint32(c.FrameDurationMs) * c.SampleRate / 1000)
}

func (c EnergyVADConfig) SilenceFrames() int {
	if c.FrameDurationMs <= 0 {
		return 15
	}
	return c.SilenceDurationMs / c.FrameDurationMs
}

type EnergyThresholdVAD struct {
	config        EnergyVADConfig
	state         VADState
	silenceCount  int
	speechStart   time.Time
	accumulated   []int16
}

func NewEnergyThresholdVAD(config EnergyVADConfig) *EnergyThresholdVAD {
	if config.SampleRate == 0 {
		config.SampleRate = 16000
	}
	if config.FrameDurationMs == 0 {
		config.FrameDurationMs = 20
	}
	if config.SilenceDurationMs == 0 {
		config.SilenceDurationMs = 300
	}
	if config.EnergyThreshold == 0 {
		config.EnergyThreshold = 0.01
	}
	return &EnergyThresholdVAD{
		config: config,
		state:  VADStateSilence,
	}
}

func (v *EnergyThresholdVAD) Reset() {
	v.state = VADStateSilence
	v.silenceCount = 0
	v.accumulated = nil
}

func (v *EnergyThresholdVAD) State() VADState {
	return v.state
}

func (v *EnergyThresholdVAD) ProcessFrame(samples []int16) VADEvent {
	energy := ComputeRMS(samples)
	now := time.Now()

	switch v.state {
	case VADStateSilence:
		if energy >= v.config.EnergyThreshold {
			v.state = VADStateSpeech
			v.silenceCount = 0
			v.speechStart = now
			return VADEvent{
				Type:      VADEventSpeechStart,
				Timestamp: now,
				Energy:    energy,
			}
		}
		return VADEvent{
			Type:      VADEventSilence,
			Timestamp: now,
			Energy:    energy,
		}

	case VADStateSpeech:
		if energy < v.config.EnergyThreshold {
			v.silenceCount++
			if v.silenceCount >= v.config.SilenceFrames() {
				v.state = VADStateSilence
				v.silenceCount = 0
				return VADEvent{
					Type:      VADEventSpeechEnd,
					Timestamp: now,
					Energy:    energy,
				}
			}
		} else {
			v.silenceCount = 0
		}
		return VADEvent{
			Type:      VADEventSilence,
			Timestamp: now,
			Energy:    energy,
		}

	default:
		return VADEvent{
			Type:      VADEventSilence,
			Timestamp: now,
			Energy:    energy,
		}
	}
}

func (v *EnergyThresholdVAD) ProcessPCM(pcmData []byte) []VADEvent {
	samples := BytesToInt16Samples(pcmData)
	return v.ProcessSamples(samples)
}

func (v *EnergyThresholdVAD) ProcessSamples(samples []int16) []VADEvent {
	frameSize := v.config.FrameSize()

	if len(v.accumulated) > 0 {
		samples = append(v.accumulated, samples...)
		v.accumulated = nil
	}

	var events []VADEvent
	for i := 0; i+frameSize <= len(samples); i += frameSize {
		frame := samples[i : i+frameSize]
		evt := v.ProcessFrame(frame)
		events = append(events, evt)
	}

	remaining := len(samples) % frameSize
	if remaining > 0 {
		start := len(samples) - remaining
		v.accumulated = make([]int16, remaining)
		copy(v.accumulated, samples[start:])
	}

	return events
}

func (v *EnergyThresholdVAD) DetectSpeech(ctx context.Context, audioData []byte) (*interfaces.VADResult, error) {
	if len(audioData) == 0 {
		return nil, interfaces.ErrEmptyAudioData
	}

	events := v.ProcessPCM(audioData)

	speechDetected := v.state == VADStateSpeech
	var confidence float64
	if speechDetected {
		confidence = 1.0
	} else {
		confidence = 0.0
		for _, evt := range events {
			if evt.Type == VADEventSpeechStart {
				confidence = 1.0
				speechDetected = true
				break
			}
		}
	}

	return &interfaces.VADResult{
		SpeechDetected: speechDetected,
		Confidence:     confidence,
		Timestamp:      time.Now(),
	}, nil
}

func ComputeRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}

	var sum float64
	for _, s := range samples {
		v := float64(s) / float64(math.MaxInt16)
		sum += v * v
	}

	return math.Sqrt(sum / float64(len(samples)))
}

func BytesToInt16Samples(data []byte) []int16 {
	numSamples := len(data) / 2
	samples := make([]int16, numSamples)
	for i := range numSamples {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return samples
}

func Int16SamplesToBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func GenerateSilence(sampleRate uint32, durationMs int) []int16 {
	numSamples := int(sampleRate) * durationMs / 1000
	return make([]int16, numSamples)
}

func GenerateTone(sampleRate uint32, frequency float64, amplitude float64, durationMs int) []int16 {
	numSamples := int(sampleRate) * durationMs / 1000
	samples := make([]int16, numSamples)
	for i := range numSamples {
		t := float64(i) / float64(sampleRate)
		v := amplitude * math.Sin(2*math.Pi*frequency*t)
		samples[i] = int16(v * float64(math.MaxInt16))
	}
	return samples
}
