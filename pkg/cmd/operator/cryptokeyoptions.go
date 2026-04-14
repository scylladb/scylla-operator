package operator

import (
	"fmt"
	"slices"
	"time"

	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	// RFC 5702: The key size of RSA/SHA-512 keys MUST NOT be less than 1024 bits and MUST NOT be more than 4096 bits.
	// NIST Special Publication 800-57 Part 3 (DOI: 10.6028) recommends a minimum of 2048-bit keys for RSA.
	rsaKeySizeMin     = 2048
	rsaKeySizeMax     = 4096
	defaultRSAKeySize = 4096

	// Recommendations (like the below) claim that a 384-bit ECDSA key is comparable with 7680 bits for RSA.
	// See https://docs.aws.amazon.com/acm/latest/userguide/acm-certificate-characteristics.html.
	defaultECDSACurveBitSize = 384

	cryptoKeyBufferSizeMinValue = 1

	cryptoKeyBufferSizeMaxFlagKey = "crypto-key-buffer-size-max"
	cryptoKeySizeFlagKey          = "crypto-key-size"
	cryptoRSAKeySizeFlagKey       = "crypto-rsa-key-size"
	cryptoECDSAKeySizeFlagKey     = "crypto-ecdsa-key-size"
)

// CryptoKeyOptions holds CLI options for cryptographic key generation.
type CryptoKeyOptions struct {
	KeyType       string
	RSAKeySize    int
	ECDSAKeySize  int
	BufferSizeMin int
	BufferSizeMax int
	BufferDelay   time.Duration
}

func DefaultCryptoKeyOptions() CryptoKeyOptions {
	return CryptoKeyOptions{
		KeyType:       string(crypto.RSAKeyType),
		RSAKeySize:    defaultRSAKeySize,
		ECDSAKeySize:  defaultECDSACurveBitSize,
		BufferSizeMin: 10,
		BufferSizeMax: 30,
		BufferDelay:   200 * time.Millisecond,
	}
}

func (o *CryptoKeyOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.KeyType, "crypto-key-type", "", o.KeyType, fmt.Sprintf("The type of cryptographic key to use for certificate generation. Supported values: %s, %s.", crypto.RSAKeyType, crypto.ECDSAKeyType))
	cmd.Flags().IntVarP(&o.RSAKeySize, cryptoRSAKeySizeFlagKey, "", o.RSAKeySize, fmt.Sprintf("The RSA key size in bits, used when --crypto-key-type=%s. Must be between %d and %d.", crypto.RSAKeyType, rsaKeySizeMin, rsaKeySizeMax))
	cmd.Flags().IntVarP(&o.ECDSAKeySize, cryptoECDSAKeySizeFlagKey, "", o.ECDSAKeySize, fmt.Sprintf("The ECDSA curve bit size, used when --crypto-key-type=%s. Must be one of %v.", crypto.ECDSAKeyType, crypto.SupportedECDSACurveBitSizes()))
	cmd.Flags().IntVarP(&o.BufferSizeMin, "crypto-key-buffer-size-min", "", o.BufferSizeMin, fmt.Sprintf("Minimal number of pre-generated crypto keys that are used for quick certificate issuance. The minimum size is %d.", cryptoKeyBufferSizeMinValue))
	cmd.Flags().IntVarP(&o.BufferSizeMax, cryptoKeyBufferSizeMaxFlagKey, "", o.BufferSizeMax, fmt.Sprintf("Maximum number of pre-generated crypto keys that are used for quick certificate issuance. The minimum size is %d. If not set, it will adjust to be at least the size of crypto-key-buffer-size-min.", cryptoKeyBufferSizeMinValue))
	cmd.Flags().DurationVarP(&o.BufferDelay, "crypto-key-buffer-delay", "", o.BufferDelay, "Delay is the time to wait when generating next certificate in the (min, max) range. Certificate generation below the min threshold is not affected.")

	// Deprecated flags.
	cmd.Flags().IntVarP(&o.RSAKeySize, cryptoKeySizeFlagKey, "", o.RSAKeySize, fmt.Sprintf("Deprecated: alias for --%s; does not affect --%s. Use --%s and/or --%s instead.", cryptoRSAKeySizeFlagKey, cryptoECDSAKeySizeFlagKey, cryptoRSAKeySizeFlagKey, cryptoECDSAKeySizeFlagKey))
	cmd.Flags().MarkDeprecated(cryptoKeySizeFlagKey, fmt.Sprintf("use --%s or --%s instead", cryptoRSAKeySizeFlagKey, cryptoECDSAKeySizeFlagKey))
}

func (o *CryptoKeyOptions) Validate() error {
	var errs []error

	switch crypto.KeyType(o.KeyType) {
	case crypto.RSAKeyType:
		if o.RSAKeySize < rsaKeySizeMin {
			errs = append(errs, fmt.Errorf("crypto-rsa-key-size must not be less than %d", rsaKeySizeMin))
		}

		if o.RSAKeySize > rsaKeySizeMax {
			errs = append(errs, fmt.Errorf("crypto-rsa-key-size must not be more than %d", rsaKeySizeMax))
		}

	case crypto.ECDSAKeyType:
		if !slices.Contains(crypto.SupportedECDSACurveBitSizes(), o.ECDSAKeySize) {
			errs = append(errs, fmt.Errorf("crypto-ecdsa-key-size must be one of %v, got: %d", crypto.SupportedECDSACurveBitSizes(), o.ECDSAKeySize))
		}

	default:
		errs = append(errs, fmt.Errorf("unsupported crypto-key-type %q, supported values are: %s, %s", o.KeyType, crypto.RSAKeyType, crypto.ECDSAKeyType))
	}

	if o.BufferSizeMin < cryptoKeyBufferSizeMinValue {
		errs = append(errs, fmt.Errorf("crypto-key-buffer-size-min (%d) has to be at least %d", o.BufferSizeMin, cryptoKeyBufferSizeMinValue))
	}

	if o.BufferSizeMax < o.BufferSizeMin {
		errs = append(errs, fmt.Errorf(
			"crypto-key-buffer-size-max (%d) can't be lower then crypto-key-buffer-size-min (%d)",
			o.BufferSizeMax,
			o.BufferSizeMin,
		))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *CryptoKeyOptions) Complete(cmd *cobra.Command) error {
	maxChanged := cmd.Flags().Lookup(cryptoKeyBufferSizeMaxFlagKey).Changed
	if !maxChanged && o.BufferSizeMin > o.BufferSizeMax {
		o.BufferSizeMax = o.BufferSizeMin
	}

	return nil
}

func (o *CryptoKeyOptions) ToKeyGeneratorConfig() crypto.KeyGeneratorConfig {
	cfg := crypto.KeyGeneratorConfig{
		Type:          crypto.KeyType(o.KeyType),
		BufferSizeMin: o.BufferSizeMin,
		BufferSizeMax: o.BufferSizeMax,
		BufferDelay:   o.BufferDelay,
	}

	switch crypto.KeyType(o.KeyType) {
	case crypto.RSAKeyType:
		cfg.KeySize = o.RSAKeySize
	case crypto.ECDSAKeyType:
		cfg.KeySize = o.ECDSAKeySize
	}

	return cfg
}
