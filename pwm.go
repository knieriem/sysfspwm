//go:build linux

// Package sysfspwm provides access to PWM channels made available by sysfs.
package sysfspwm

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/knieriem/sysfspwm/internal/devattr"
)

const sysfsPrefix = "/sys/class/pwm/pwmchip"

type Channel struct {
	enable    *devattr.File
	period    *devattr.File
	dutyCycle *devattr.File
}

// OpenChannel returns a handle corresponding to the
// PWM channel referenced by the arguments chip and channel,
// making use of the following pattern:
//
//	/sys/class/pwm/pwmchip<chip>/pwm<channel>
//
// In case the sysfs directory for the specified channel is
// not present yet, this function will try to make it available first,
// by writing to the device's "export" file.
func OpenChannel(chip, channel int) (*Channel, error) {
	dir, err := chanDir(chip, channel)
	if err != nil {
		return nil, err
	}

	ch := new(Channel)
	ch.enable, err = devattr.Open(dir, "enable", os.O_RDWR)
	if err != nil {
		return nil, err
	}
	ch.dutyCycle, err = devattr.Open(dir, "duty_cycle", os.O_RDWR)
	if err != nil {
		ch.enable.Close()
		return nil, err
	}
	ch.period, err = devattr.Open(dir, "period", os.O_WRONLY)
	if err != nil {
		ch.enable.Close()
		ch.dutyCycle.Close()
		return nil, err
	}

	i, err := ch.enable.ReadInt()
	if err != nil {
		ch.Close()
		return nil, err
	}
	if i != 0 {
		// If the channel is already enabled, reset the duty cycle
		// as preparation for further steps.
		err = ch.dutyCycle.Write0()
		if err != nil {
			ch.Close()
			return nil, err
		}
	}
	return ch, err
}

func chanDir(chip, channel int) (string, error) {
	devDir := sysfsPrefix + strconv.Itoa(chip)
	d := filepath.Join(devDir, "pwm"+strconv.Itoa(channel))
	if fi, err := os.Stat(d); err == nil && fi.IsDir() {
		return d, nil
	}

	numChan, err := devattr.ReadIntFile(devDir, "npwm")
	if err != nil {
		return "", err
	}
	if channel >= numChan {
		return "", fmt.Errorf("pwmchip%d: channel index (%d) exceeds number of channels (%d)", chip, channel, numChan)
	}
	err = devattr.WriteIntFile(devDir, "export", channel)
	if err != nil {
		return "", err
	}
	retries := 20
	for retries > 0 {
		time.Sleep(100 * time.Millisecond)
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d, nil
		}
		retries--
	}
	return "", fmt.Errorf("pwmchip%d: could not export channel %d", chip, channel)
}

const (
	// DutyMax refers to a duty cycle of 100%
	DutyMax int32 = 1 << 24
)

// PWM configures the channel's frequency and duty cycle.
// The function signature has been aligned with [periph.io/x/conn/v3/gpio.PinOut.PWM]:
//   - A value for the duty argument may be in the range [0, [DutyMax]],
//     with the maximum value corresponding to 100%.
//   - The freq argument has a resolution of 1 millihertz.
func (ch *Channel) PWM(duty int32, freq int64) error {
	if freq == 0 {
		return ch.enable.Write0()
	}
	tf := 1000 * time.Second / time.Duration(freq)
	period := tf.Nanoseconds()
	dutyCycle := period * int64(duty) >> 24
	if dutyCycle > period {
		dutyCycle = period
	}
	if ch.dutyCycle.Int64() > period {
		ch.dutyCycle.Write0()
	}
	ch.period.WriteInt64(period)
	ch.dutyCycle.WriteInt64(dutyCycle)
	if ch.enable.IsZero() {
		ch.enable.Write1()
	}
	return nil
}

// Close calls close on the channel's underlying
// sysfs attribute files "enable", "duty_cycle", and "period".
func (ch *Channel) Close() error {
	err := ch.enable.Close()
	if err1 := ch.dutyCycle.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err1 := ch.period.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}
