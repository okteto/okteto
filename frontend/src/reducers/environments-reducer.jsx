const initialEnvironmentsState = {
  byId: {},
  isFetching: false
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
        isFetching: false
      };
    }
    case 'HANDLE_FETCH_ERROR': {
      return {
        ...state,
        isFetching: false
      }
    }
    default: return state;
  }
};
