package systemd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
)

var ErrNotExist = errors.New("unit does not exist")

type SystemdControl struct {
	conn *dbus.Conn
}

func transformSystemdError(err error) error {
	var godbusErr godbus.Error
	if errors.As(err, &godbusErr) {
		switch godbusErr.Name {
		case "org.freedesktop.systemd1.NoSuchUnit":
			return ErrNotExist
		default:
			return err
		}
	}

	return err
}

func newSystemdControl(conn *dbus.Conn) (*SystemdControl, error) {
	return &SystemdControl{
		conn: conn,
	}, nil
}
func NewSystemdSystemControl(ctx context.Context) (*SystemdControl, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't create dbus connection to systemd: %w", err)
	}

	return newSystemdControl(conn)
}

func NewSystemdUserControl(ctx context.Context) (*SystemdControl, error) {
	conn, err := dbus.NewUserConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't create dbus connection to user's systemd: %w", err)
	}

	return newSystemdControl(conn)
}

func (c *SystemdControl) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *SystemdControl) DaemonReload(ctx context.Context) error {
	err := c.conn.ReloadContext(ctx)
	if err != nil {
		return fmt.Errorf("can't reload systemd configs: %w", err)
	}

	return nil
}

func (c *SystemdControl) EnableUnits(ctx context.Context, unitFiles []string) error {
	_, _, err := c.conn.EnableUnitFilesContext(ctx, unitFiles, false, false)
	if err != nil {
		return fmt.Errorf("can't enable units %q: %w", strings.Join(unitFiles, ", "), transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) DisableUnits(ctx context.Context, unitFiles []string) error {
	_, err := c.conn.DisableUnitFilesContext(ctx, unitFiles, false)
	if err != nil {
		return fmt.Errorf("can't disable units %q: %w", strings.Join(unitFiles, ", "), transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) EnableUnit(ctx context.Context, unitFile string) error {
	return c.EnableUnits(ctx, []string{unitFile})
}

func (c *SystemdControl) DisableUnit(ctx context.Context, unitFile string) error {
	return c.DisableUnits(ctx, []string{unitFile})
}

func (c *SystemdControl) StartUnit(ctx context.Context, unitFile string) error {
	_, err := c.conn.StartUnitContext(ctx, unitFile, "replace", nil)
	if err != nil {
		return fmt.Errorf("can't start unit %q: %w", unitFile, transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) StopUnit(ctx context.Context, unitFile string) error {
	_, err := c.conn.StopUnitContext(ctx, unitFile, "replace", nil)
	if err != nil {
		return fmt.Errorf("can't stop unit %q: %w", unitFile, transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) EnableAndStartUnit(ctx context.Context, unitFile string) error {
	err := c.EnableUnit(ctx, unitFile)
	if err != nil {
		return err
	}

	err = c.StartUnit(ctx, unitFile)
	if err != nil {
		return err
	}

	return nil
}

func (c *SystemdControl) DisableAndStopUnit(ctx context.Context, unitFile string) error {
	err := c.DisableUnit(ctx, unitFile)
	if err != nil {
		return err
	}

	err = c.StopUnit(ctx, unitFile)
	if err != nil {
		return err
	}

	return nil
}
