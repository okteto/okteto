const initialSpacesState = {
  list: [],
  current: null,
  isFetching: false,
  isLoaded: false
};

export default (state = initialSpacesState, action) => {
  switch (action.type) {
    case 'REQUEST_SPACES': {
      return {
        ...state, 
        isFetching: true
      };
    }
    case 'RECEIVE_SPACES': {
      return {
        ...state,
        list: action.spaces,
        isFetching: false
      };
    }
    case 'RECEIVE_SELECTED_SPACE': {
      return {
        ...state,
        current: action.space,
        isFetching: false,
        isLoaded: true
      };
    }
    case 'FAILED_RECEIVE_SPACES': 
    case 'FAILED_RECEIVE_SPACE': {
      return state;
    }
    default: return state;
  }
};
