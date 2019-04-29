const initialEnvironmentsState = {
  byId: {},
  isFetching: false,
  isLoaded: false
};

export default (state = initialEnvironmentsState, action) => {
  switch (action.type) {
    case 'REQUEST_ENVIRONMENTS': {
      return {
        ...state,
        isFetching: true
      };
    }
    case 'RECEIVE_ENVIRONMENTS': {
      return {
        ...state,
        byId: Array.from(action.environments).reduce((map, environment) => {
          map[environment.id] = environment;
          return map;
        }, {}),
        isFetching: false,
        isLoaded: true
      };
    }
    case 'FAILED_RECEIVE_ENVIRONMENTS': {
      return {
        ...initialEnvironmentsState,
        isLoaded: true
      }
    }
    default: return state;
  }
};
