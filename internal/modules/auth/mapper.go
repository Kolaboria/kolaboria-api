package auth

func ToProfileResponse(profile *Profile) *ProfileResponse {
	if profile == nil {
		return nil
	}
	return &ProfileResponse{
		AccountType: profile.AccountType,
		Avatar:      profile.Avatar,
		Headline:    profile.Headline,
		Bio:         profile.Bio,
		Location:    profile.Location,
		IsVerified:  profile.IsVerified,
		UpdatedAt:   profile.UpdatedAt,
	}
}

func ToUserResponse(user *User) UserResponse {
	if user == nil {
		return UserResponse{}
	}
	return UserResponse{
		ID:              user.ID,
		Name:            user.GetFullName(),
		Username:        user.GetUsername(),
		Email:           user.Email,
		EmailVerifiedAt: user.EmailVerifiedAt,
		IsActive:        user.IsActive,
		SystemRole:      user.EffectiveSystemRole(),
		CreatedAt:       user.CreatedAt,
		Profile:         ToProfileResponse(user.Profile),
	}
}

func ToUserResponseWithoutProfile(user *User) UserResponse {
	if user == nil {
		return UserResponse{}
	}
	return UserResponse{
		ID:              user.ID,
		Name:            user.GetFullName(),
		Username:        user.GetUsername(),
		Email:           user.Email,
		EmailVerifiedAt: user.EmailVerifiedAt,
		IsActive:        user.IsActive,
		SystemRole:      user.EffectiveSystemRole(),
		CreatedAt:       user.CreatedAt,
		Profile:         nil,
	}
}

func ToAuthResponse(user *User, accessToken, refreshToken string) AuthResponse {
	return AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         ToUserResponseWithoutProfile(user),
	}
}

func ToLoginResponse(user *User, accessToken, refreshToken string) LoginResponse {
	if user == nil {
		return LoginResponse{}
	}
	return LoginResponse{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		Name:            user.GetFullName(),
		Email:           user.Email,
		EmailVerifiedAt: user.EmailVerifiedAt,
		SystemRole:      user.EffectiveSystemRole(),
	}
}

func ToCurrentUserProfileResp(profile *Profile) *CurrentUserProfileResp {
	if profile == nil {
		return nil
	}
	return &CurrentUserProfileResp{
		AccountType: profile.AccountType,
		Avatar:      profile.Avatar,
		Headline:    profile.Headline,
		Bio:         profile.Bio,
		Location:    profile.Location,
		IsVerified:  profile.IsVerified,
		UpdatedAt:   profile.UpdatedAt,
	}
}

func ToCurrentUserResponse(user *User) CurrentUserResponse {
	if user == nil {
		return CurrentUserResponse{}
	}
	return CurrentUserResponse{
		ID:                     user.ID,
		Username:               user.GetUsername(),
		SystemRole:             user.EffectiveSystemRole(),
		InitiableOrganizations: make([]InitiableOrganizationResponse, 0),
		Name:                   user.GetFullName(),
		Email:                  user.Email,
		EmailVerifiedAt:        user.EmailVerifiedAt,
		IsActive:               user.IsActive,
		CreatedAt:              user.CreatedAt,
		Profile:                ToCurrentUserProfileResp(user.Profile),
	}
}
