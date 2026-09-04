package runtime

func ApplyRegion(profileChanged bool, profile string, region string, setProfile func(string), setRegion func(string)) {
	if profileChanged && setProfile != nil {
		setProfile(profile)
	}
	if setRegion != nil {
		setRegion(region)
	}
}
