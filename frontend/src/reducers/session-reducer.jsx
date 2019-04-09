const initialSessionState = {
  user: {},
  isAuthenticated: true // false // Change once Authentication is done.
};

export default (state = initialSessionState, action) => {
  switch (action.type) {
    case 'AUTH_SUCCESS': {
      return action.user ? {
        user: action.user,
        isAuthenticated: true
      } : {
        ...initialSessionState
      };
    }
    case 'LOGOUT': {
      return {...initialSessionState};
    }
    case 'UPDATE_SESSION': {
      return {
        ...state,
        user: {
          ...state.user,
          ...action.user
        }
      };
    }
    case 'SAVE_SESSION': {
      localStorage.setItem('session', JSON.stringify(state || {}));
      return state;
    }
    case 'AFTER_RESTORE_SESSION': {
      return {...action.session};
    }
    default: {
      return state;
    }
  }
};