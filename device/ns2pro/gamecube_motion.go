package ns2pro

import (
	"encoding/binary"
	"math"
	"time"
)

const (
	gameCubeMotionSize                = 40
	gameCubeTick                      = 1250 * time.Microsecond
	gameCubeGyroCountsPerDegreeSecond = 16.4
)

// The normal 0x0A IMU layout and 800 Hz clock were documented from physical
// GameCube captures by chyyran. Presentation owns the clock and history, so
// repeated control reads cannot fabricate rotation or advance the sensor.
type gameCubeMotionEncoder struct {
	active      bool
	lastTick    int64
	accel, gyro [3]int16
	orientation [4]float64
	report      [gameCubeMotionSize]byte
}

func (s InputState) gameCubeHasMotion() bool {
	return s.AccelX != 0 || s.AccelY != 0 || s.AccelZ != 0 ||
		s.GyroX != 0 || s.GyroY != 0 || s.GyroZ != 0
}

func (m *gameCubeMotionEncoder) present(s InputState, age time.Duration) *[gameCubeMotionSize]byte {
	if !s.gameCubeHasMotion() {
		*m = gameCubeMotionEncoder{}
		return nil
	}

	tick := int64(age / gameCubeTick)
	accel := [3]int16{s.AccelX, s.AccelY, s.AccelZ}
	gyro := [3]int16{s.GyroX, s.GyroY, s.GyroZ}
	elapsed := tick - m.lastTick
	if !m.active || elapsed < 0 || elapsed > int64(100*time.Millisecond/gameCubeTick) {
		m.orientation = [4]float64{1, 0, 0, 0}
		m.accel, m.gyro = accel, gyro
		elapsed = 0
	}
	clear(m.report[:])
	binary.LittleEndian.PutUint32(m.report[:4], uint32(tick&0xfff)|uint32(elapsed)<<12|0x0e000000)
	body := m.report[4:]

	accelRange := gameCubeMotionRange(m.accel, accel)
	gyroRange := gameCubeMotionRange(m.gyro, gyro)
	if accelRange == 1 {
		body[0] |= 1
	}
	if gyroRange == 1 {
		body[0] |= 2
	}
	m.writeOrientation(body)
	for i := range accel {
		// Interpolate the documented history points between the previous
		// presentation and the current sample. Only the endpoint is measured.
		earlier := (2*int(m.accel[i]) + int(accel[i])) / 3
		middle := (int(m.accel[i]) + 2*int(accel[i])) / 3
		earlierGyro := (int(m.gyro[i]) + 2*int(gyro[i])) / 3
		putGameCubeBits(body, 68+i*14, 14, gameCubePackedSample(earlier, accelRange, 14))
		putGameCubeBits(body, 110+i*13, 13, gameCubePackedSample(earlierGyro, gyroRange*2, 13))
		putGameCubeBits(body, 149+i*13, 13, gameCubePackedSample(middle, accelRange*2, 13))
		putGameCubeBits(body, 188+i*14, 14, gameCubePackedSample(int(gyro[i]), gyroRange, 14))
		putGameCubeBits(body, 230+i*14, 14, gameCubePackedSample(int(accel[i]), accelRange, 14))
	}
	// The orientation in each report describes the preceding sample time.
	// Advance only after encoding it. The source supplies no temperature.
	m.integrate(gyro, float64(time.Duration(elapsed)*gameCubeTick)/float64(time.Second))
	m.active, m.lastTick, m.accel, m.gyro = true, tick, accel, gyro
	return &m.report
}

func gameCubeMotionRange(previous, current [3]int16) int {
	for i := range previous {
		if previous[i] < -8192 || previous[i] > 8191 ||
			current[i] < -8192 || current[i] > 8191 {
			return 4
		}
	}
	return 1
}

func gameCubePackedSample(value, scale, width int) uint32 {
	limit := 1 << (width - 1)
	packed := int(math.Round(float64(value) / float64(scale)))
	return uint32(min(max(packed, -limit), limit-1))
}

func (m *gameCubeMotionEncoder) writeOrientation(body []byte) {
	q := m.orientation
	face := 0
	for i := 1; i < len(q); i++ {
		if math.Abs(q[i]) > math.Abs(q[face]) {
			face = i
		}
	}
	putGameCubeBits(body, 2, 2, uint32(face))
	const bias = 1 << 20
	for i := 0; i < 3; i++ {
		coordinate := int(math.Round(q[(face+i+1)&3]/q[face]*bias)) + bias
		putGameCubeBits(body, 5+21*i, 21, uint32(min(max(coordinate, 0), 2*bias-1)))
	}
}

func (m *gameCubeMotionEncoder) integrate(gyro [3]int16, seconds float64) {
	x := (float64(m.gyro[0]) + float64(gyro[0])) / 2
	y := (float64(m.gyro[1]) + float64(gyro[1])) / 2
	z := (float64(m.gyro[2]) + float64(gyro[2])) / 2
	length := math.Sqrt(x*x + y*y + z*z)
	if length == 0 || seconds == 0 {
		return
	}
	angle := length / gameCubeGyroCountsPerDegreeSecond * math.Pi / 180 * seconds / 2
	w, factor := math.Cos(angle), math.Sin(angle)/length
	x, y, z = x*factor, y*factor, z*factor
	q := m.orientation
	next := [4]float64{
		q[0]*w - q[1]*x - q[2]*y - q[3]*z,
		q[0]*x + q[1]*w + q[2]*z - q[3]*y,
		q[0]*y - q[1]*z + q[2]*w + q[3]*x,
		q[0]*z + q[1]*y - q[2]*x + q[3]*w,
	}
	norm := math.Sqrt(next[0]*next[0] + next[1]*next[1] + next[2]*next[2] + next[3]*next[3])
	for i := range next {
		m.orientation[i] = next[i] / norm
	}
}

func putGameCubeBits(dst []byte, offset, width int, value uint32) {
	for width > 0 {
		shift := offset & 7
		count := min(width, 8-shift)
		mask := uint32(1<<count) - 1
		dst[offset/8] |= byte((value & mask) << shift)
		offset += count
		width -= count
		value >>= count
	}
}
