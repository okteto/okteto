import { combineReducers } from 'redux';

import sessionReducer from 'reducers/session-reducer';
import spacesReducer from 'reducers/spaces-reducer';

export default combineReducers({
  spaces: spacesReducer,
  session: sessionReducer
});