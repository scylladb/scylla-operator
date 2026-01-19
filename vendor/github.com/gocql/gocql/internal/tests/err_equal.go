package tests

func ErrEqual(err1, err2 error) bool {
	if err1 != nil && err2 != nil {
		return err1.Error() == err2.Error()
	}
	return err1 == nil && err2 == nil
}
